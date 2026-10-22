package modelManager

import (
	"context"
	"errors"
	"fmt"
	"nstudio/app/common/eventManager"
	"nstudio/app/common/response"
	"nstudio/app/common/status"
	"nstudio/app/common/util"
	"nstudio/app/config"
	"nstudio/app/enums/Engines"
	tts "nstudio/app/tts/engine"
	"nstudio/app/tts/engine/elevenlabs"
	"nstudio/app/tts/engine/gemini"
	"nstudio/app/tts/engine/google"
	"nstudio/app/tts/engine/mssapi5"
	"nstudio/app/tts/engine/openai"
	"nstudio/app/tts/engine/piper"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
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

		initSpeaker()

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

	entry, exists := manager.Engines[engineName]
	if !exists {
		manager.Unlock()
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
				response.Err("Failed to stop model %s:%s: %w", engineName, modelID, err)
			}
		}

		for _, instance := range modelPool.Instances {
			if err := instance.Start(modelID); err != nil {
				response.Err("Failed to restart model %s:%s: %w", engineName, modelID, err)
			}
		}
	}

	if entry.Engine.Engine != nil {
		entry.Engine.Models = entry.Engine.Engine.FetchModels()
		manager.Engines[engineName] = entry
	}

	manager.Unlock()

	_ = RefreshModels()
}

func restartLocalEngines() {
	manager.RLock()
	var localEngineIDs []string
	for engineID, entry := range manager.Engines {
		if entry.Engine.Type == tts.Local {
			localEngineIDs = append(localEngineIDs, engineID)
		}
	}
	manager.RUnlock()

	for _, engineID := range localEngineIDs {
		restartEngine(engineID)
	}
}

func createModelPool(engineID, modelID string) (*ModelPool, error) {
	instanceCount := 1
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
		case string(Engines.MsSapi5):
			engine = &mssapi5.MsSapi5{}
		case string(Engines.ElevenLabs):
			engine = &elevenlabs.ElevenLabs{}
		case string(Engines.OpenAI):
			engine = &openai.OpenAI{}
		case string(Engines.Google):
			engine = &google.Google{}
		case string(Engines.Gemini):
			engine = &gemini.Gemini{}
		default:
			engineEntry, exists := manager.Engines[engineID]
			if !exists || engineEntry.Engine.NewInstance == nil {
				return nil, response.Err("unknown engine: %s", engineID)
			}
			engine = engineEntry.Engine.NewInstance()
		}

		if err := engine.Initialize(); err != nil {
			return nil, response.Err("%w", err)
		}

		instances = append(instances, engine)
	}

	log.Info(fmt.Sprintf("Created %d instances for %s/%s", instanceCount, engineID, modelID))

	return newModelPool(instances), nil
}

func RefreshModels() error {
	toggles := config.GetEngineToggles()
	manager.Lock()
	if manager.released {
		manager.Unlock()
		return nil
	}

	enabledModels := 0
	for engineID, models := range toggles {
		for modelID, enabled := range models {
			if entry, exists := manager.Engines[engineID]; exists {
				if modelPool, modelExists := entry.Models[modelID]; modelExists {
					for _, instance := range modelPool.Instances {
						if enabled {
							err := instance.Start(modelID)
							if err != nil {
								response.Err("Failed to start model %s: %w", modelID, err)
							}
						} else {
							err := instance.Stop(modelID)
							if err != nil {
								response.Err("Failed to stop model %s: %w", modelID, err)
							}
						}
					}
					if enabled {
						enabledModels++
					}
				} else if enabled {
					if _, modelKnown := entry.Engine.Models[modelID]; !modelKnown {
						continue
					}

					pool, err := createModelPool(engineID, modelID)
					if err != nil {
						response.Err("Failed to create pool for %s/%s: %w", engineID, modelID, err)
						continue
					}

					entry.Models[modelID] = pool

					for _, instance := range pool.Instances {
						if err := instance.Start(modelID); err != nil {
							response.Err("Failed to start model %s: %w", modelID, err)
						}
					}

					enabledModels++
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

var releaseDrainTimeout = 30 * time.Second

// Draining happens outside the lock: an in-flight request may still need
// GetEngineInstance before it can hand its instance back.
func ReleaseAll() {
	manager.Lock()
	manager.released = true
	detached := make(map[string]map[string]*ModelPool, len(manager.Engines))
	for engineID, entry := range manager.Engines {
		if len(entry.Models) == 0 {
			continue
		}
		detached[engineID] = entry.Models
		entry.Models = make(map[string]*ModelPool)
		for _, pool := range detached[engineID] {
			pool.detach()
		}
	}
	manager.Unlock()

	for engineID, pools := range detached {
		for modelID, pool := range pools {
			recovered := pool.drain(releaseDrainTimeout)
			stopInstances(engineID, modelID, recovered)

			if inUse := len(pool.Instances) - len(recovered); inUse > 0 {
				response.Err("Released %s:%s with %d of %d instances still in use; they will be stopped when they finish",
					engineID, modelID, inUse, len(pool.Instances))
				go stopInstancesWhenReturned(engineID, modelID, pool, inUse)
			}
		}
	}
}

func stopInstancesWhenReturned(engineID, modelID string, pool *ModelPool, count int) {
	for range count {
		stopInstances(engineID, modelID, []tts.Base{<-pool.pool})
	}
}

func stopInstances(engineID, modelID string, instances []tts.Base) {
	for _, instance := range instances {
		if err := instance.Stop(modelID); err != nil {
			response.Err("Failed to stop model %s:%s: %w", engineID, modelID, err)
		}
	}
}

func Reacquire() error {
	manager.Lock()
	manager.released = false
	manager.Unlock()

	return RefreshModels()
}

func IsReleased() bool {
	manager.RLock()
	defer manager.RUnlock()
	return manager.released
}

func ReloadModels() error {
	manager.Lock()

	for engineID, entry := range manager.Engines {
		if entry.Engine.Engine != nil {
			entry.Engine.Models = entry.Engine.Engine.FetchModels()
		}
		manager.Engines[engineID] = entry
	}

	manager.Unlock()
	return RefreshModels()
}

func RegisterEngine(baseEngine tts.Engine) error {
	manager.Lock()
	defer manager.Unlock()

	engineID := baseEngine.ID

	entry := &EngineEntry{
		Engine: baseEngine,
		Models: make(map[string]*ModelPool),
	}
	manager.Engines[engineID] = entry

	toggles := config.GetEngineToggles()

	for modelID := range baseEngine.Models {
		modelEnabled := false
		if engineToggles, exists := toggles[engineID]; exists {
			if enabled, modelExists := engineToggles[modelID]; modelExists {
				modelEnabled = enabled
			}
		}

		if !modelEnabled || manager.released {
			continue
		}

		pool, err := createModelPool(engineID, modelID)
		if err != nil {
			response.Err("Failed to create pool for %s/%s: %w", engineID, modelID, err)
			continue
		}

		entry.Models[modelID] = pool
	}

	for _, model := range baseEngine.Models {
		RegisterModel(model)
	}

	return nil
}

func UnregisterEngine(engineID string) error {
	manager.Lock()
	defer manager.Unlock()

	engineEntry, exists := manager.Engines[engineID]
	if !exists {
		return nil
	}

	for modelID, pool := range engineEntry.Models {
		for _, instance := range pool.Instances {
			if err := instance.Stop(modelID); err != nil {
				response.Err("%w", err)
			}
		}
	}

	delete(manager.Engines, engineID)
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
					response.Err("Failed to start model %s: %w", model.ID, err)
				}
			}
		}
	}
}

func GetEngineInstance(engineID, modelID string) (tts.Base, func(), bool) {
	pool, ok := findOrCreatePool(engineID, modelID)
	if !ok {
		return nil, nil, false
	}

	instance, ok := pool.GetNext()
	if !ok {
		return nil, nil, false
	}

	return instance, func() { pool.Release(instance) }, true
}

func findOrCreatePool(engineID, modelID string) (*ModelPool, bool) {
	manager.RLock()
	pool, found := manager.findPoolLocked(engineID, modelID)
	released := manager.released
	manager.RUnlock()

	if released {
		return nil, false
	}
	if found {
		return pool, true
	}

	manager.Lock()
	defer manager.Unlock()

	if manager.released {
		return nil, false
	}
	if pool, found := manager.findPoolLocked(engineID, modelID); found {
		return pool, true
	}

	entry, ok := manager.Engines[engineID]
	if !ok {
		return nil, false
	}
	if _, modelExists := entry.Engine.Models[modelID]; !modelExists {
		return nil, false
	}

	pool, err := createModelPool(engineID, modelID)
	if err != nil {
		return nil, false
	}
	entry.Models[modelID] = pool

	return pool, true
}

func (manager *modelManager) findPoolLocked(engineID, modelID string) (*ModelPool, bool) {
	entry, ok := manager.Engines[engineID]
	if !ok {
		return nil, false
	}

	pool, ok := entry.Models[modelID]
	return pool, ok
}

func GetAllEngines() []tts.Engine {
	toggles := config.GetEngineToggles()
	var availableEngines []tts.Engine

	manager.RLock()
	defer manager.RUnlock()

	for engineID, entry := range manager.Engines {
		filteredEngine := tts.Engine{
			ID:     engineID,
			Name:   entry.Engine.Name,
			Type:   entry.Engine.Type,
			Tags:   entry.Engine.Tags,
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

	tts.SortEngines(availableEngines)
	return availableEngines
}

func CheckApiEngines(ctx context.Context) map[string]ApiAvailability {
	checkers := make(map[string]tts.AvailabilityChecker)
	manager.RLock()
	for engineID, entry := range manager.Engines {
		if entry.Engine.Type != tts.Api {
			continue
		}
		if checker, canCheck := entry.Engine.Engine.(tts.AvailabilityChecker); canCheck {
			checkers[engineID] = checker
		}
	}
	manager.RUnlock()

	availability := make(map[string]ApiAvailability, len(checkers))
	var availabilityMutex sync.Mutex
	var checks sync.WaitGroup
	for engineID, checker := range checkers {
		checks.Go(func() {
			err := checker.CheckAvailability(ctx)
			result := ApiAvailability{Available: err == nil, MissingApiKey: errors.Is(err, tts.ErrApiKeyNotSet)}
			if err != nil {
				result.Reason = err.Error()
			}
			availabilityMutex.Lock()
			availability[engineID] = result
			availabilityMutex.Unlock()
		})
	}
	checks.Wait()
	return availability
}

func GetActiveEngines() []tts.Engine {
	manager.RLock()
	defer manager.RUnlock()

	var activeEngines []tts.Engine

	for engineID, entry := range manager.Engines {
		activeEngine := tts.Engine{
			ID:     engineID,
			Name:   entry.Engine.Name,
			Type:   entry.Engine.Type,
			Tags:   entry.Engine.Tags,
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

	tts.SortEngines(activeEngines)
	return activeEngines
}

func GetAllModels() map[string]tts.Model {
	result := make(map[string]tts.Model)

	manager.RLock()
	defer manager.RUnlock()

	for _, entry := range manager.Engines {
		for _, model := range entry.Engine.Models {
			result[model.Engine+":"+model.ID] = model
		}
	}

	return result
}

func GetModelVoices(engineName string, modelID string) ([]tts.Voice, error) {
	manager.RLock()
	selectedEngine, exists := manager.Engines[engineName]
	var voiceSource tts.Base
	if exists {
		if modelPool, modelExists := selectedEngine.Models[modelID]; modelExists && len(modelPool.Instances) > 0 {
			voiceSource = modelPool.Instances[0]
		} else if _, modelExists := selectedEngine.Engine.Models[modelID]; modelExists {
			voiceSource = selectedEngine.Engine.Engine
		}
	}
	manager.RUnlock()

	if !exists {
		return nil, response.Err("Engine %s not found", engineName)
	}
	if voiceSource == nil {
		return nil, response.Err("Model %s not found for engine %s", modelID, engineName)
	}

	return voiceSource.GetVoices(modelID)
}

func GetInstanceCount(engineID string, modelID string) int {
	manager.RLock()
	defer manager.RUnlock()

	if pool, found := manager.findPoolLocked(engineID, modelID); found {
		return len(pool.Instances)
	}
	return 0
}
