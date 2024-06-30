package voiceManager

import (
	"encoding/json"
	"fmt"
	"math/rand"
	tts "nstudio/app/tts/engine"
	"nstudio/app/tts/util"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type CharacterVoice struct {
	Name   string `json:"name"`
	Engine string `json:"engine"`
	Model  string `json:"model"`
	Voice  string `json:"voice"`
}

func (characterVoice *CharacterVoice) UnmarshalJSON(data []byte) error {
	// Define a temporary structure to decode your JSON data
	type Alias CharacterVoice
	unmarshalTarget := &struct {
		*Alias
	}{
		Alias: (*Alias)(characterVoice),
	}
	if err := json.Unmarshal(data, &unmarshalTarget); err != nil {
		return err
	}
	// Here you could add additional handling if necessary
	return nil
}

func (characterVoice CharacterVoice) MarshalJSON() ([]byte, error) {
	type Alias CharacterVoice
	return json.Marshal(&struct {
		Key string `json:"key"`
		Alias
	}{
		Key:   fmt.Sprintf("%s:%s", characterVoice.Engine, characterVoice.Model),
		Alias: (Alias)(characterVoice),
	})
}

type VoiceManager struct {
	sync.Mutex
	Engines         map[string]tts.Engine
	Models          map[string]tts.Model
	CharacterVoices map[string]CharacterVoice
}

var (
	instance *VoiceManager
	once     sync.Once
)

func GetInstance() *VoiceManager {
	once.Do(func() {
		instance = &VoiceManager{
			Engines:         make(map[string]tts.Engine),
			CharacterVoices: make(map[string]CharacterVoice),
		}

		instance.LoadCharacterVoices()

	})
	return instance
}

func (manager *VoiceManager) LoadCharacterVoices() {
	executablePath, err := os.Executable()
	if err != nil {
		panic("Failed to get executable path: " + err.Error())
	}

	voiceConfigPath := filepath.Join(filepath.Dir(executablePath), "voiceConfig.json")

	file, err := os.ReadFile(voiceConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If the file does not exist, create it with an empty JSON array
			err = os.WriteFile(voiceConfigPath, []byte("[]"), 0644)
			if err != nil {
				panic("Failed to create voice config file: " + err.Error())
			}
			file = []byte("[]") // Set file to empty JSON array to prevent json.Unmarshal error
		} else {
			panic("Failed to open voice  config file: " + err.Error())
		}
	}

	// Unmarshal JSON data into a slice of CharacterVoice
	var voices []CharacterVoice
	err = json.Unmarshal(file, &voices)
	if err != nil {
		panic("Failed to unmarshal voice config: " + err.Error())
	}

	manager.CharacterVoices = make(map[string]CharacterVoice)
	for _, voice := range voices {
		manager.CharacterVoices[voice.Name] = voice
	}
}

func (manager *VoiceManager) UpdateCharacterVoices(data string) {
	var newVoices []CharacterVoice
	err := json.Unmarshal([]byte(data), &newVoices)
	if err != nil {
		panic("Failed to unmarshal voices: " + err.Error())
	}

	manager.CharacterVoices = make(map[string]CharacterVoice)
	for _, voice := range newVoices {
		manager.CharacterVoices[voice.Name] = voice
	}

	executablePath, err := os.Executable()
	if err != nil {
		panic("Failed to get executable path: " + err.Error())
	}

	voiceConfigPath := filepath.Join(filepath.Dir(executablePath), "voiceConfig.json")

	byteData := []byte(data)

	if err := os.WriteFile(voiceConfigPath, byteData, 0644); err != nil {
		panic("Failed to write updated voice config file: " + err.Error())
	}
}

func (manager *VoiceManager) GetVoice(name string, save bool) CharacterVoice {
	manager.Lock()
	defer manager.Unlock()

	if _, ok := manager.CharacterVoices[name]; !ok {
		engine := manager.calculateEngine(name)
		model, voice := manager.calculateVoice(engine, name)

		characterVoice := CharacterVoice{
			Name:   name,
			Engine: engine,
			Model:  model,
			Voice:  voice,
		}
		if save {
			manager.SaveVoice(name, characterVoice)
		}
		return characterVoice
	}

	return manager.CharacterVoices[name]
}

func (manager *VoiceManager) SaveVoice(name string, voice CharacterVoice) {
	manager.CharacterVoices[name] = voice

	voicesArray := make([]CharacterVoice, 0, len(manager.CharacterVoices))
	for _, v := range manager.CharacterVoices {
		voicesArray = append(voicesArray, v)
	}

	data, err := json.Marshal(voicesArray)
	if err != nil {
		panic("Failed to marshal voices: " + err.Error())
	}

	executablePath, err := os.Executable()
	if err != nil {
		panic("Failed to get executable path: " + err.Error())
	}
	voiceConfigPath := filepath.Join(filepath.Dir(executablePath), "voiceConfig.json")

	err = os.WriteFile(voiceConfigPath, data, 0644)
	if err != nil {
		panic("Failed to write to voice config file: " + err.Error())
	}
}

func (manager *VoiceManager) calculateEngine(value string) string {
	return "piper" //TODO add proper engine selection
}

func (manager *VoiceManager) calculateVoice(engineID string, value string) (string, string) {
	if strings.Contains(value, ":") {
		segments := strings.Split(value, ":")

		if len(segments) < 2 {
			panic("Failed to parse voice name: " + value)
		}

		return segments[0], segments[1]
	} else {
		selectedEngine, _ := manager.GetEngine(engineID)

		seed := int64(0)
		for _, r := range value {
			seed = seed*31 + int64(r)
		}

		rand.Seed(seed)

		models := make([]string, 0, len(selectedEngine.Models))

		for model := range selectedEngine.Models {
			models = append(models, model)
		}
		if len(models) == 0 {
			panic("No models available")
		}
		selectedModel := models[rand.Intn(len(models)-1)]

		voices, _ := selectedEngine.Engine.GetVoices(selectedModel)
		if len(voices) == 0 {
			panic("No voices available: " + engineID)
		}
		selectedVoice := voices[rand.Intn(len(voices)-1)]

		return selectedModel, selectedVoice.ID
	}
}

func (manager *VoiceManager) RegisterEngine(newEngine tts.Engine) {
	models := util.GetKeys(newEngine.Models)
	err := newEngine.Engine.Initialize(models)
	if err != nil {
		panic("error initializing engine:" + err.Error())
	}
	manager.Lock()
	defer manager.Unlock()

	//for _, model := range newEngine.Models {
	//	manager.RegisterModel(model)
	//}

	manager.Engines[newEngine.ID] = newEngine
}

func (manager *VoiceManager) GetEngine(ID string) (tts.Engine, bool) {
	selectedEngine, ok := manager.Engines[ID]
	return selectedEngine, ok
}

func (manager *VoiceManager) UnregisterEngine(name string) {
	manager.Lock()
	defer manager.Unlock()

	delete(manager.Engines, name)
}

func (manager *VoiceManager) GetEngines() []tts.Engine {
	var allEngines []tts.Engine
	for _, managerEngine := range manager.Engines {
		allEngines = append(allEngines, managerEngine)
	}
	return allEngines
}

func (manager *VoiceManager) GetVoices(engineName string, model string) ([]tts.Voice, error) {
	voiceEngine, exists := manager.Engines[engineName]

	if !exists {
		return nil, fmt.Errorf("engine %s does not exist", engineName)
	}

	return voiceEngine.Engine.GetVoices(model)
}
