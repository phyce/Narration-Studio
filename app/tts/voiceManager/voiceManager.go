package voiceManager

import (
	"fmt"
	"math/rand"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/util"
	"strings"
	"sync"
)

type CharacterVoice struct {
	Name   string `json:"name"`
	Engine string `json:"engine"`
	Model  string `json:"model"`
	Voice  string `json:"voice"`
}

type VoiceManager struct {
	sync.Mutex
	Engines         map[string]engine.Engine
	Models          map[string]engine.Model
	CharacterVoices map[string]CharacterVoice
}

var (
	instance *VoiceManager
	once     sync.Once
)

func GetInstance() *VoiceManager {
	once.Do(func() {
		instance = &VoiceManager{
			Engines:         make(map[string]engine.Engine),
			CharacterVoices: make(map[string]CharacterVoice),
		}
	})
	return instance
}

func (manager *VoiceManager) GetVoice(name string) CharacterVoice {
	manager.Lock()
	defer manager.Unlock()

	if _, ok := manager.CharacterVoices[name]; !ok {
		engine := manager.calculateEngine(name)
		model, voice := manager.calculateVoice(engine, name)
		manager.CharacterVoices[name] = CharacterVoice{
			Name:   name,
			Engine: engine,
			Model:  model,
			Voice:  voice,
		}
	}

	return manager.CharacterVoices[name]
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
		selectedModel := models[rand.Intn(len(models))]

		voices, _ := selectedEngine.Engine.GetVoices(selectedModel)
		if len(voices) == 0 {
			panic("No voices available")
		}
		selectedVoice := voices[rand.Intn(len(voices))]

		return selectedModel, selectedVoice.ID
	}
}

func (manager *VoiceManager) RegisterEngine(newEngine engine.Engine) {
	models := util.GetKeys(newEngine.Models)
	err := newEngine.Engine.Initialize(models)
	if err != nil {
		fmt.Println("error initializing engine")
		fmt.Println(err)
	}
	manager.Lock()
	defer manager.Unlock()

	//for _, model := range newEngine.Models {
	//	manager.RegisterModel(model)
	//}

	manager.Engines[newEngine.ID] = newEngine
}

func (manager *VoiceManager) GetEngine(ID string) (engine.Engine, bool) {
	selectedEngine, ok := manager.Engines[ID]
	return selectedEngine, ok
}

func (manager *VoiceManager) UnregisterEngine(name string) {
	manager.Lock()
	defer manager.Unlock()

	delete(manager.Engines, name)
}

func (manager *VoiceManager) GetEngines() []engine.Engine {
	var allEngines []engine.Engine
	for _, managerEngine := range manager.Engines {
		allEngines = append(allEngines, managerEngine)
	}
	return allEngines
}

func (manager *VoiceManager) GetVoices(engineName string, model string) ([]engine.Voice, error) {
	voiceEngine, exists := manager.Engines[engineName]
	fmt.Println(manager.Engines)
	fmt.Println("/Engines done")
	fmt.Println(engineName)
	fmt.Print("manager:")
	fmt.Println(manager.Engines[engineName])
	fmt.Print("voiceEngine:")
	fmt.Println(voiceEngine)
	if !exists {
		return nil, fmt.Errorf("engine %s does not exist", engineName)
	}

	return voiceEngine.Engine.GetVoices(model)
}

//func selectVoice(character string) Voice {
//	voices := getAllVoices()
//	seed := hashStringToUint64(character)
//	rand.Seed(int64(seed))
//	randomIndex := rand.Intn(len(voices))
//	return voices[randomIndex]
//}
//
//func hashStringToUint64(text string) uint64 {
//	hash := fnv.New64a()
//	_, err := hash.Write([]byte(text))
//	if err != nil {
//		return uint64(time.Now().UnixNano())
//	}
//	return hash.Sum64()
//}
