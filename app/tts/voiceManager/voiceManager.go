package voiceManager

import (
	"encoding/json"
	"fmt"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/speaker"
	"math/rand"
	"nstudio/app/common/response"
	"nstudio/app/common/status"
	"nstudio/app/common/util"
	"nstudio/app/config"
	tts "nstudio/app/tts/engine"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type VoiceManager struct {
	sync.Mutex
	Engines         map[string]tts.Engine
	CharacterVoices map[string]util.CharacterVoice
	AllocatedVoices map[string]util.CharacterVoice //These ones do not get saved permanently, only per run
}

var (
	instance *VoiceManager
	once     sync.Once
)

func GetInstance() *VoiceManager {
	once.Do(func() {
		instance = &VoiceManager{
			Engines:         make(map[string]tts.Engine),
			CharacterVoices: make(map[string]util.CharacterVoice),
			AllocatedVoices: make(map[string]util.CharacterVoice),
		}

		instance.LoadCharacterVoices()

		format := beep.Format{
			SampleRate:  48000,
			NumChannels: 1,
			Precision:   2,
		}

		if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
			response.Error(response.Data{
				Summary: "failed to initialize speaker",
				Detail:  err.Error(),
			})
		}

	})
	return instance
}

func (manager *VoiceManager) LoadCharacterVoices() {
	voiceConfigPath := filepath.Join(config.GetInstance().GetConfigPath(), "voiceConfig.json")

	file, err := os.ReadFile(voiceConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.WriteFile(voiceConfigPath, []byte("[]"), 0644)
			if err != nil {
				panic("Failed to create voice config file: " + err.Error())
			}
			file = []byte("[]")
		} else {
			panic("Failed to open voice  config file: " + err.Error())
		}
	}

	var voices map[string]util.CharacterVoice
	//var voices []CharacterVoice
	fmt.Println("string(file)")
	fmt.Println(string(file))
	err = json.Unmarshal(file, &voices)
	if err != nil {
		panic("Failed to unmarshal voice config: " + err.Error())
	}

	//manager.CharacterVoices = voices
	for _, voice := range voices {
		manager.CharacterVoices[voice.Name] = voice
	}
}

func (manager *VoiceManager) UpdateCharacterVoices(data string) error {
	var newVoices map[string]util.CharacterVoice
	err := json.Unmarshal([]byte(data), &newVoices)
	if err != nil {
		return util.TraceError(err)
	}

	manager.CharacterVoices = newVoices

	voiceConfigPath := filepath.Join(config.GetInstance().GetConfigPath(), "voiceConfig.json")

	byteData := []byte(data)

	if err := os.WriteFile(voiceConfigPath, byteData, 0644); err != nil {
		return util.TraceError(err)
	}

	return nil
}

func (manager *VoiceManager) GetVoice(name string, save bool) (util.CharacterVoice, error) {
	manager.Lock()
	defer manager.Unlock()

	if allocatedVoice, exists := manager.AllocatedVoices[name]; exists {
		return allocatedVoice, nil
	}

	fmt.Println("Getting Voice for: ", name)

	if strings.HasPrefix(name, "::") {
		parts := strings.Split(name, ":")
		if len(parts) == 5 {
			characterVoice := util.CharacterVoice{
				Name:   "",
				Engine: parts[2],
				Model:  parts[3],
				Voice:  parts[4],
			}
			return characterVoice, nil
		} else {
			return util.CharacterVoice{}, util.TraceError(
				fmt.Errorf("invalid line could not be processed: " + name),
			)
		}
	}

	if voice, ok := manager.CharacterVoices[name]; ok {

		modelToggles := config.GetInstance().GetModelToggles()

		if modelToggles[voice.Engine][voice.Model] {

			return voice, nil
		}
	}

	engine := manager.calculateEngine(name)
	model, voice, err := manager.calculateVoice(engine, name)
	if err != nil {
		return util.CharacterVoice{}, util.TraceError(err)
	}

	fmt.Println("engine, model, voice, name")
	fmt.Println(engine, model, voice, name)

	characterVoice := util.CharacterVoice{
		Name:   name,
		Engine: engine,
		Model:  model,
		Voice:  voice,
	}

	if save {
		err = manager.SaveVoice(name, characterVoice)
		if err != nil {
			return util.CharacterVoice{}, util.TraceError(err)
		}
	} else {
		manager.AllocatedVoices[name] = characterVoice
	}

	return characterVoice, nil
}

func (manager *VoiceManager) SaveVoice(name string, voice util.CharacterVoice) error {
	manager.CharacterVoices[name] = voice

	voicesArray := make([]util.CharacterVoice, 0, len(manager.CharacterVoices))
	for _, v := range manager.CharacterVoices {
		voicesArray = append(voicesArray, v)
	}

	data, err := json.Marshal(voicesArray)
	if err != nil {
		return util.TraceError(err)
	}

	voiceConfigPath := filepath.Join(config.GetInstance().GetConfigPath(), "voiceConfig.json")

	err = os.WriteFile(voiceConfigPath, data, 0644)
	if err != nil {
		return util.TraceError(err)
	}
	return nil
}

func (manager *VoiceManager) calculateEngine(name string) string {
	response.Debug(response.Data{
		Summary: "Getting engine for: " + name,
	})

	voice, exists := manager.CharacterVoices[name]
	if exists {
		enabled, exists := config.GetInstance().GetModelToggles()[voice.Engine][voice.Model]
		if exists && enabled {
			return voice.Engine
		}
	}

	seed := int64(0)
	for _, r := range name {
		seed = seed*31 + int64(r)
	}
	rand.Seed(seed)

	engines := make([]string, 0, len(manager.Engines))
	for engine := range manager.Engines {
		engines = append(engines, engine)
	}

	if len(engines) == 0 {
		util.TraceError(fmt.Errorf("No engines found"))
		return ""
	} else if len(engines) == 1 {
		return engines[0]
	} else {
		selectedEngine := engines[rand.Intn(len(engines)-1)]
		return selectedEngine
	}
}

func (manager *VoiceManager) calculateVoice(engineID string, name string) (string, string, error) {
	if strings.Contains(name, ":") {
		segments := strings.Split(name, ":")

		if len(segments) < 2 {
			return "", "", util.TraceError(fmt.Errorf("Failed to parse voice name:" + name))
		}

		return segments[0], segments[1], nil
	} else {
		selectedEngine, _ := manager.GetEngine(engineID)

		seed := int64(0)
		for _, r := range name {
			seed = seed*31 + int64(r)
		}
		rand.Seed(seed)

		modelToggles := config.GetInstance().GetModelToggles()

		models := make([]string, 0, len(selectedEngine.Models))
		for modelID, _ := range selectedEngine.Models {
			if modelToggles[engineID][modelID] {
				models = append(models, modelID)
			}
		}

		var selectedModel string

		if len(models) == 0 {
			return "", "", util.TraceError(
				fmt.Errorf("No enabled models found for engine %s", selectedEngine),
			)
		} else if len(models) == 1 {
			selectedModel = models[0]
		} else {
			selectedModel = models[rand.Intn(len(models)-1)]
		}

		voices, _ := selectedEngine.Engine.GetVoices(selectedModel)
		if len(voices) == 0 {
			return "", "", util.TraceError(
				fmt.Errorf("No voices found for engine %s", selectedEngine),
			)
		}
		selectedVoice := voices[rand.Intn(len(voices)-1)]

		return selectedModel, selectedVoice.ID, nil
	}
}

func (manager *VoiceManager) RegisterEngine(newEngine tts.Engine) error {
	manager.Lock()
	defer manager.Unlock()

	manager.Engines[newEngine.ID] = newEngine
	err := manager.Engines[newEngine.ID].Engine.Initialize()
	if err != nil {
		return util.TraceError(err)
	}

	for _, model := range newEngine.Models {
		manager.RegisterModel(model)
	}

	manager.RefreshModels()

	return nil
}

func (manager *VoiceManager) RegisterModel(model tts.Model) {
	toggles := config.GetInstance().GetModelToggles()

	engine := manager.Engines[model.Engine]

	_ /*enabled*/, exists := toggles[model.Engine][model.ID]
	if !exists {
		fmt.Println("New Model: ", model.Engine, model.ID)
	} else {
		fmt.Println("Already existing model: ", model.Engine, model.Name)
	}

	engine.Models[model.ID] = model

	modelToggles := config.GetInstance().GetModelToggles()

	if modelToggles[model.Engine][model.ID] {
		err := engine.Engine.Start(model.ID)
		if err != nil {
			response.Error(response.Data{
				Summary: "Failed to prepare piper model:" + model.ID,
				Detail:  err.Error(),
			})
		}
	}
}

func (manager *VoiceManager) GetEngine(ID string) (tts.Engine, bool) {
	selectedEngine, ok := manager.Engines[ID]
	return selectedEngine, ok
}

func (manager *VoiceManager) GetEngines() []tts.Engine {
	toggles := config.GetInstance().GetModelToggles()
	var availableEngines []tts.Engine

	for _, managerEngine := range manager.Engines {
		filteredEngine := tts.Engine{
			ID:     managerEngine.ID,
			Name:   managerEngine.Name,
			Models: make(map[string]tts.Model),
		}

		for modelName, model := range managerEngine.Models {
			engineToggles, engineExists := toggles[managerEngine.ID]
			if engineExists {
				if enabled, toggleExists := engineToggles[modelName]; toggleExists && enabled {
					filteredEngine.Models[modelName] = tts.Model{
						ID:       model.ID,
						Name:     model.Name,
						Engine:   model.Engine,
						Download: model.Download,
					}
				}
			} else {
				filteredEngine.Models[modelName] = tts.Model{
					ID:       model.ID,
					Name:     model.Name,
					Engine:   model.Engine,
					Download: model.Download,
				}
			}
		}

		availableEngines = append(availableEngines, filteredEngine)
	}

	return availableEngines
}

func (manager *VoiceManager) GetAllModels() map[string]tts.Model {
	result := make(map[string]tts.Model)

	for _, engine := range manager.Engines {
		for _, model := range engine.Models {
			result[model.ID] = model
		}
	}

	return result
}

func (manager *VoiceManager) GetVoices(engineName string, model string) ([]tts.Voice, error) {
	voiceEngine, exists := manager.Engines[engineName]

	if !exists {
		return nil, util.TraceError(fmt.Errorf("Engine %s not found", engineName))
	}

	return voiceEngine.Engine.GetVoices(model)
}

func (manager *VoiceManager) RefreshModels() {
	toggles := config.GetInstance().GetModelToggles()

	enabledModels := 0
	for engine, models := range toggles {
		for model, enabled := range models {
			if _, exists := manager.Engines[engine]; exists {
				if enabled {
					manager.Engines[engine].Engine.Start(model)
					enabledModels++
				} else {
					manager.Engines[engine].Engine.Stop(model)
				}
			}
		}
	}

	if enabledModels > 0 {
		status.Set(status.Ready, "")
	} else {
		status.Set(status.Error, "No models enabled")
	}
}

func (manager *VoiceManager) ReloadModels() {
	for name, engine := range manager.Engines {
		engine.Models = engine.Engine.FetchModels()
		manager.Engines[name] = engine
	}
	manager.RefreshModels()
}

func (manager *VoiceManager) ResetAllocatedVoices() {
	manager.AllocatedVoices = make(map[string]util.CharacterVoice)
}
