package modelManager

import (
	"fmt"
	"nstudio/app/common/eventManager"
	"nstudio/app/common/response"
	"nstudio/app/common/status"
	"nstudio/app/common/util"
	"nstudio/app/config"
	"nstudio/app/enums/Engines"
	tts "nstudio/app/tts/engine"
	"nstudio/app/tts/engine/elevenlabs"
	"nstudio/app/tts/engine/mssapi4"
	"nstudio/app/tts/engine/openai"
	"nstudio/app/tts/engine/piper"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/speaker"
)

var (
	manager *modelManager
	once    sync.Once
)

func Initialize(isGUIMode bool) {
	once.Do(func() {
		manager = &modelManager{
			Engines:   make(map[string]*EngineEntry),
			IsGUIMode: isGUIMode,
		}

		//TODO: Move to audio module
		format := beep.Format{
			SampleRate:  48000,
			NumChannels: 1,
			Precision:   2,
		}

		if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
			response.Error(util.MessageData{
				Summary: "failed to initialize speaker",
				Detail:  err.Error(),
			})
		}

		eventManager.GetInstance().SubscribeToEvent("config.changed", handleConfigChange)
	})
}

func handleConfigChange(data interface{}) {
	changeData, ok := data.(map[string]interface{})
	if !ok {
		return
	}

	path, pathOk := changeData["path"].(string)
	if !pathOk || path == "" {
		restartLocalEngines()
		return
	}

	if !strings.HasPrefix(path, "engine.local.") {
		return
	}

	parts := strings.Split(path, ".")
	if len(parts) < 3 {
		return
	}

	engineName := strings.ToLower(parts[2])
	log.Info(fmt.Sprintf("Config changed for engine: %s, path: %s", engineName, path))

	restartEngine(engineName)
}

func restartEngine(engineName string) {
	manager.Lock()
	defer manager.Unlock()

	entry, exists := manager.Engines[engineName]
	if !exists {
		log.Warn(fmt.Sprintf("Engine %s not found, skipping restart", engineName))
		return
	}

	log.Info(fmt.Sprintf("Restarting \"%s\" engine models", engineName))

	for modelID, modelPool := range entry.Models {
		if len(modelPool.Instances) == 0 {
			continue
		}

		for _, instance := range modelPool.Instances {
			if err := instance.Stop(modelID); err != nil {
				response.Error(util.MessageData{
					Summary: fmt.Sprintf("Failed to stop model %s:%s", engineName, modelID),
					Detail:  err.Error(),
				})
			}
		}

		for _, instance := range modelPool.Instances {
			if err := instance.Start(modelID); err != nil {
				response.Error(util.MessageData{
					Summary: fmt.Sprintf("Failed to restart model %s:%s", engineName, modelID),
					Detail:  err.Error(),
				})
			}
		}
	}
}

func restartLocalEngines() {
	manager.Lock()
	defer manager.Unlock()

	for engineID, entry := range manager.Engines {
		if entry.Engine.Type == tts.Local {
			manager.Unlock()
			restartEngine(engineID)
			manager.Lock()
		}
	}
}

func RefreshModels() error {
	toggles := config.GetEngineToggles()
	manager.Lock()

	enabledModels := 0
	for engineID, models := range toggles {
		for modelID, enabled := range models {
			if entry, exists := manager.Engines[engineID]; exists {
				if modelPool, modelExists := entry.Models[modelID]; modelExists {
					for _, instance := range modelPool.Instances {
						if enabled {
							err := instance.Start(modelID)
							if err != nil {
								response.Err(err)
								response.Error(util.MessageData{
									Summary: "Failed to start model:" + modelID,
									Detail:  err.Error(),
								})
							}
						} else {
							err := instance.Stop(modelID)
							if err != nil {
								response.Err(err)
								response.Debug(util.MessageData{
									Summary: "Failed to stop model:" + modelID,
									Detail:  err.Error(),
								})
							}
						}
					}
					if enabled {
						enabledModels++
					}
				}
			}
		}
	}
	manager.Unlock()

	if enabledModels > 0 {
		status.Set(status.Ready, "")
		return nil
	}

	status.Set(status.Warning, "No models enabled")
	return response.NewWarn("No models enabled")
}

func ReloadModels() {
	manager.Lock()

	for engineID, entry := range manager.Engines {
		if len(entry.Models) > 0 {
			for _, modelPool := range entry.Models {
				if len(modelPool.Instances) > 0 {
					models := modelPool.Instances[0].FetchModels()
					entry.Engine.Models = models
					break
				}
			}
		}
		manager.Engines[engineID] = entry
	}

	manager.Unlock()
	RefreshModels()
}

func RegisterEngine(baseEngine tts.Engine) error {
	manager.Lock()
	defer manager.Unlock()

	engineID := baseEngine.ID

	entry := &EngineEntry{
		Engine: baseEngine,
		Models: make(map[string]*ModelPool),
	}

	toggles := config.GetEngineToggles()

	for modelID := range baseEngine.Models {
		modelEnabled := false
		if engineToggles, exists := toggles[engineID]; exists {
			if enabled, modelExists := engineToggles[modelID]; modelExists {
				modelEnabled = enabled
			}
		}

		if !modelEnabled {
			//response.Debug
			//log.Debug(fmt.Sprintf("Skipping instance creation for disabled model: %s/%s", engineID, modelID))
			continue
		}

		instanceCount := 1 // Default for GUI mode

		if !manager.IsGUIMode {
			configuredCount := config.GetServerInstanceCount(engineID, modelID)
			if configuredCount > 0 {
				instanceCount = configuredCount
			}
		}

		var instances []tts.Base
		for i := 0; i < instanceCount; i++ {
			var engine tts.Base

			switch engineID {
			case string(Engines.Piper):
				engine = &piper.Piper{}
			case string(Engines.MsSapi4):
				engine = &mssapi4.MsSapi4{}
			case string(Engines.ElevenLabs):
				engine = &elevenlabs.ElevenLabs{}
			case string(Engines.OpenAI):
				engine = &openai.OpenAI{}
			default:
				engine = baseEngine.Engine
			}

			if err := engine.Initialize(); err != nil {
				return response.Err(err)
			}

			instances = append(instances, engine)
		}

		// Create buffered channel pool and populate with all instances
		poolChan := make(chan tts.Base, instanceCount)
		for _, instance := range instances {
			poolChan <- instance
		}

		entry.Models[modelID] = &ModelPool{
			Instances: instances,
			pool:      poolChan,
		}

		log.Info(fmt.Sprintf("Created %d instances for %s/%s", instanceCount, engineID, modelID))
	}

	manager.Engines[engineID] = entry

	for _, model := range baseEngine.Models {
		RegisterModel(model)
	}

	return nil
}

func RegisterModel(model tts.Model) {
	toggles := config.GetEngineToggles()

	entry := manager.Engines[model.Engine]
	if entry == nil {
		return
	}

	_ /*enabled*/, exists := toggles[model.Engine][model.ID]
	if !exists {
		response.Debug(util.MessageData{
			Summary: "New Model: " + model.Engine + ":" + model.Name,
		})
	} else {
		response.Debug(util.MessageData{
			Summary: "Already existing model: " + model.Engine + ":" + model.Name,
		})
	}

	entry.Engine.Models[model.ID] = model

	modelToggles := config.GetEngineToggles()
	if modelToggles[model.Engine][model.ID] {
		if modelPool, exists := entry.Models[model.ID]; exists {
			for _, instance := range modelPool.Instances {
				err := instance.Start(model.ID)
				if err != nil {
					response.Error(util.MessageData{
						Summary: "Failed to start model:" + model.ID,
						Detail:  err.Error(),
					})
				}
			}
		}
	}
}

func GetEngineInstance(engineID, modelID string) (tts.Base, func(), bool) {
	manager.RLock()
	entry, ok := manager.Engines[engineID]
	manager.RUnlock()
	if !ok {
		return nil, nil, false
	}

	instance, pool, ok := entry.GetModelInstance(modelID)
	if !ok {
		return nil, nil, false
	}

	releaseFunc := func() {
		pool.Release(instance)
	}

	return instance, releaseFunc, true
}

func GetAllEngines() []tts.Engine {
	toggles := config.GetEngineToggles()
	var availableEngines []tts.Engine

	for engineID, entry := range manager.Engines {
		filteredEngine := tts.Engine{
			ID:     engineID,
			Name:   entry.Engine.Name,
			Models: make(map[string]tts.Model),
		}

		for modelName, model := range entry.Engine.Models {
			engineToggles, engineExists := toggles[engineID]
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

func GetActiveEngines() []tts.Engine {
	manager.RLock()
	defer manager.RUnlock()

	var activeEngines []tts.Engine

	for engineID, entry := range manager.Engines {
		activeEngine := tts.Engine{
			ID:     engineID,
			Name:   entry.Engine.Name,
			Models: make(map[string]tts.Model),
		}

		for modelID, modelPool := range entry.Models {
			if len(modelPool.Instances) > 0 {
				if model, exists := entry.Engine.Models[modelID]; exists {
					activeEngine.Models[modelID] = tts.Model{
						ID:       model.ID,
						Name:     model.Name,
						Engine:   model.Engine,
						Download: model.Download,
					}
				}
			}
		}

		if len(activeEngine.Models) > 0 {
			activeEngines = append(activeEngines, activeEngine)
		}
	}

	return activeEngines
}

func GetAllModels() map[string]tts.Model {
	result := make(map[string]tts.Model)

	for _, entry := range manager.Engines {
		for _, model := range entry.Engine.Models {
			result[model.ID] = model
		}
	}

	return result
}

func GetModelVoices(engineName string, modelID string) ([]tts.Voice, error) {
	entry, exists := manager.Engines[engineName]

	if !exists {
		return nil, response.Err(fmt.Errorf("Engine %s not found", engineName))
	}

	if modelPool, modelExists := entry.Models[modelID]; modelExists && len(modelPool.Instances) > 0 {
		return modelPool.Instances[0].GetVoices(modelID)
	}

	return nil, response.Err(fmt.Errorf("Model %s not found for engine %s", modelID, engineName))
}

func GetInstanceCount(engineID string, modelID string) int {
	if entry, exists := manager.Engines[engineID]; exists {
		if modelPool, modelExists := entry.Models[modelID]; modelExists {
			return len(modelPool.Instances)
		}
	}
	return 0
}
