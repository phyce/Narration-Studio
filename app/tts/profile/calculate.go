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
