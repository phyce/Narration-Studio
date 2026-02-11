package profile

import (
	"fmt"
	"nstudio/app/common/response"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/modelManager"
	"strings"

	"golang.org/x/exp/rand"
)

func setRandSeed(name string) {
	seed := uint64(0)
	for _, r := range name {
		seed = seed*31 + uint64(r)
	}
	rand.Seed(seed)
}

func calculateEngine(name string) (engine.Engine, error) {
	setRandSeed(name)

	managerEngines := modelManager.GetAllEngines()
	var enabledEngines []engine.Engine

	for _, managerEngine := range managerEngines {
		for _, enabled := range config.GetEngineToggles()[managerEngine.ID] {
			if enabled {
				enabledEngines = append(enabledEngines, managerEngine)
				break
			}
		}
	}

	if len(enabledEngines) == 0 {
		return engine.Engine{}, response.NewWarn("No enabled engines found")
	} else if len(enabledEngines) == 1 {
		return enabledEngines[0], nil
	} else {
		selectedEngine := enabledEngines[rand.Intn(len(enabledEngines)-1)]
		return selectedEngine, nil
	}
}

func calculateVoice(engine engine.Engine, name string) (string, string, error) {
	//If the name contains a colon that means an override was provided
	if strings.Contains(name, ":") {
		segments := strings.Split(name, ":")

		if len(segments) < 2 {
			return "", "", response.Err(fmt.Errorf("Failed to parse voice name:" + name))
		}

		return segments[0], segments[1], nil
	}

	modelToggles := config.GetEngineToggles()

	models := make([]string, 0, len(engine.Models))
	for modelID, _ := range engine.Models {
		if modelToggles[engine.ID][modelID] {
			models = append(models, modelID)
		}
	}
	var selectedModel string

	if len(models) == 0 {
		return "", "", response.NewWarn(fmt.Sprintf("No enabled models found for engine: %s", engine.Name))
	} else if len(models) == 1 {
		selectedModel = models[0]
	} else {
		selectedModel = models[rand.Intn(len(models)-1)]
	}

	voices, err := modelManager.GetModelVoices(engine.ID, selectedModel)

	if err != nil || len(voices) == 0 {
		return "", "", response.Err(
			fmt.Errorf("No voices found for engine: %s", engine.Name),
		)
	}
	selectedVoice := voices[rand.Intn(len(voices)-1)]

	return selectedModel, selectedVoice.ID, nil
}

func getEngineTogglesFromFlat(toggles map[string]bool) map[string]map[string]bool {
	engineToggles := make(map[string]map[string]bool)

	for key, value := range toggles {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		engineID := parts[0]
		modelID := parts[1]

		if _, exists := engineToggles[engineID]; !exists {
			engineToggles[engineID] = make(map[string]bool)
		}

		engineToggles[engineID][modelID] = value
	}

	return engineToggles
}

func calculateProfileEngine(name string, profileID string) (engine.Engine, error) {
	setRandSeed(name)

	manager := GetManager()
	profile, err := manager.GetProfile(profileID)
	if err != nil {
		return engine.Engine{}, response.Err(err)
	}

	profileToggles := profile.GetModelToggles()
	if profileToggles == nil || len(profileToggles) == 0 {
		return calculateEngine(name)
	}

	engineToggles := getEngineTogglesFromFlat(profileToggles)

	managerEngines := modelManager.GetAllEngines()
	var enabledEngines []engine.Engine

	for _, managerEngine := range managerEngines {
		if engineModels, exists := engineToggles[managerEngine.ID]; exists {
			for _, enabled := range engineModels {
				if enabled {
					enabledEngines = append(enabledEngines, managerEngine)
					break
				}
			}
		}
	}

	if len(enabledEngines) == 0 {
		return engine.Engine{}, response.NewWarn("No enabled engines found for profile")
	} else if len(enabledEngines) == 1 {
		return enabledEngines[0], nil
	} else {
		selectedEngine := enabledEngines[rand.Intn(len(enabledEngines)-1)]
		return selectedEngine, nil
	}
}

func calculateProfileVoice(eng engine.Engine, name string, profileID string) (string, string, error) {
	if strings.Contains(name, ":") {
		segments := strings.Split(name, ":")

		if len(segments) < 2 {
			return "", "", response.Err(fmt.Errorf("Failed to parse voice name:" + name))
		}

		return segments[0], segments[1], nil
	}

	manager := GetManager()
	profile, err := manager.GetProfile(profileID)
	if err != nil {
		return "", "", response.Err(err)
	}

	profileToggles := profile.GetModelToggles()
	if profileToggles == nil || len(profileToggles) == 0 {
		return calculateVoice(eng, name)
	}

	engineToggles := getEngineTogglesFromFlat(profileToggles)

	models := make([]string, 0, len(eng.Models))
	for modelID := range eng.Models {
		if engineModels, exists := engineToggles[eng.ID]; exists {
			if engineModels[modelID] {
				models = append(models, modelID)
			}
		}
	}

	var selectedModel string

	if len(models) == 0 {
		return "", "", response.NewWarn(fmt.Sprintf("No enabled models found for engine: %s in profile", eng.Name))
	} else if len(models) == 1 {
		selectedModel = models[0]
	} else {
		selectedModel = models[rand.Intn(len(models)-1)]
	}

	voices, err := modelManager.GetModelVoices(eng.ID, selectedModel)

	if err != nil || len(voices) == 0 {
		return "", "", response.Err(
			fmt.Errorf("No voices found for engine: %s", eng.Name),
		)
	}
	selectedVoice := voices[rand.Intn(len(voices)-1)]

	return selectedModel, selectedVoice.ID, nil
}
