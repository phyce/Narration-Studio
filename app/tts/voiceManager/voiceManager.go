package voiceManager

import (
	"encoding/json"
	"fmt"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/speaker"
	"nstudio/app/common/issue"
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

var (
	manager *VoiceManager
	once    sync.Once
)

func Initialize() {
	once.Do(func() {
		manager = &VoiceManager{
			Engines:         make(map[string]tts.Engine),
			CharacterVoices: make(map[string]util.CharacterVoice),
			AllocatedVoices: make(map[string]util.CharacterVoice),
		}

		LoadCharacterVoices()

		format := beep.Format{
			SampleRate:  48000,
			NumChannels: 1,
			Precision:   2,
		}

		//TODO: Move to audio module
		if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
			response.Error(response.Data{
				Summary: "failed to initialize speaker",
				Detail:  err.Error(),
			})
		}
	})
}

func LoadCharacterVoices() {
	manager.Lock()
	defer manager.Unlock()

	voiceConfigPath := filepath.Join(config.GetConfigPath(), "voiceConfig.json")

	file, err := os.ReadFile(voiceConfigPath)

	if err != nil {
		if os.IsNotExist(err) {
			err = os.WriteFile(voiceConfigPath, []byte("{}"), 0644)
			if err != nil {
				issue.Panic("Failed to create voice config file", err)
			}
			file = []byte("{}")
		} else {
			issue.Panic("Failed to open voice  config file: ", err)
		}
	}

	var voices map[string]util.CharacterVoice
	err = json.Unmarshal(file, &voices)
	if err != nil {
		issue.Panic("Failed to unmarshal voice config: ", err)
	}

	for _, voice := range voices {
		manager.CharacterVoices[voice.Name] = voice
	}
}

// TODO maybe accept struct instead of string?
func SaveCharacterVoices(data string) error {
	manager.Lock()
	defer manager.Unlock()

	var newVoices map[string]util.CharacterVoice
	err := json.Unmarshal([]byte(data), &newVoices)
	if err != nil {
		return issue.Trace(err)
	}

	manager.CharacterVoices = newVoices

	voiceConfigPath := filepath.Join(config.GetConfigPath(), "voiceConfig.json")

	byteData := []byte(data)

	if err := os.WriteFile(voiceConfigPath, byteData, 0644); err != nil {
		return issue.Trace(err)
	}

	return nil
}

func ResetAllocatedVoices() {
	manager.AllocatedVoices = make(map[string]util.CharacterVoice)
}

func RefreshModels() {
	toggles := config.GetEngineToggles()

	enabledModels := 0
	for engine, models := range toggles {
		for model, enabled := range models {
			if _, exists := manager.Engines[engine]; exists {
				if enabled {
					err := manager.Engines[engine].Engine.Start(model)
					if err != nil {
						issue.Trace(err)
						response.Error(response.Data{
							Summary: "Failed to start piper model:" + model,
							Detail:  err.Error(),
						})
					}
					enabledModels++
				} else {
					err := manager.Engines[engine].Engine.Stop(model)
					if err != nil {
						issue.Trace(err)
						response.Error(response.Data{
							Summary: "Failed to stop piper model:" + model,
							Detail:  err.Error(),
						})
					}
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

func ReloadModels() {
	for name, engine := range manager.Engines {
		engine.Models = engine.Engine.FetchModels()
		manager.Engines[name] = engine
	}
	RefreshModels()
}

func RegisterEngine(newEngine tts.Engine) error {
	manager.Lock()
	defer manager.Unlock()

	manager.Engines[newEngine.ID] = newEngine
	err := manager.Engines[newEngine.ID].Engine.Initialize()
	if err != nil {
		return issue.Trace(err)
	}

	for _, model := range newEngine.Models {
		RegisterModel(model)
	}

	RefreshModels()

	return nil
}

func RegisterModel(model tts.Model) {
	toggles := config.GetEngineToggles()

	engine := manager.Engines[model.Engine]

	_ /*enabled*/, exists := toggles[model.Engine][model.ID]
	if !exists {
		response.Debug(response.Data{
			Summary: "New Model: " + model.Engine + ":" + model.Name,
		})
	} else {
		response.Debug(response.Data{
			Summary: "Already existing model: " + model.Engine + ":" + model.Name,
		})
	}

	engine.Models[model.ID] = model

	modelToggles := config.GetEngineToggles()

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

func SaveVoice(name string, voice util.CharacterVoice) error {
	manager.CharacterVoices[name] = voice

	data, err := json.Marshal(manager.CharacterVoices)
	if err != nil {
		return issue.Trace(err)
	}

	voiceConfigPath := filepath.Join(config.GetConfigPath(), "voiceConfig.json")

	err = os.WriteFile(voiceConfigPath, data, 0644)
	if err != nil {
		return issue.Trace(err)
	}
	return nil
}

func GetVoice(name string, save bool) (util.CharacterVoice, error) {
	manager.Lock()
	defer manager.Unlock()

	if allocatedVoice, exists := manager.AllocatedVoices[name]; exists {
		return allocatedVoice, nil
	}

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
			return util.CharacterVoice{}, issue.Trace(
				fmt.Errorf("invalid line could not be processed: " + name),
			)
		}
	}

	if voice, ok := manager.CharacterVoices[name]; ok {

		modelToggles := config.GetEngineToggles()

		if modelToggles[voice.Engine][voice.Model] {

			return voice, nil
		}
	}

	engine := calculateEngine(name)
	model, voice, err := calculateVoice(engine, name)
	if err != nil {
		return util.CharacterVoice{}, issue.Trace(err)
	}

	characterVoice := util.CharacterVoice{
		Name:   name,
		Engine: engine,
		Model:  model,
		Voice:  voice,
	}

	if save {
		err = SaveVoice(name, characterVoice)
		if err != nil {
			return util.CharacterVoice{}, issue.Trace(err)
		}
	} else {
		manager.AllocatedVoices[name] = characterVoice
	}

	return characterVoice, nil
}

func GetEngine(ID string) (tts.Engine, bool) {
	selectedEngine, ok := manager.Engines[ID]
	return selectedEngine, ok
}

func GetEngines() []tts.Engine {
	toggles := config.GetEngineToggles()
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

func GetAllModels() map[string]tts.Model {
	result := make(map[string]tts.Model)

	for _, engine := range manager.Engines {
		for _, model := range engine.Models {
			result[model.ID] = model
		}
	}

	return result
}

func GetVoices(engineName string, model string) ([]tts.Voice, error) {
	voiceEngine, exists := manager.Engines[engineName]

	if !exists {
		return nil, issue.Trace(fmt.Errorf("Engine %s not found", engineName))
	}

	return voiceEngine.Engine.GetVoices(model)
}

func GetCharacterVoices() map[string]util.CharacterVoice {
	return manager.CharacterVoices
}
