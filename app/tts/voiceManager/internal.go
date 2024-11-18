package voiceManager

import (
	"fmt"
	"math/rand"
	"nstudio/app/common/issue"
	"nstudio/app/common/response"
	"nstudio/app/config"
	"strings"
)

func calculateEngine(name string) string {
	response.Debug(response.Data{
		Summary: "Getting engine for: " + name,
	})

	voice, exists := manager.CharacterVoices[name]
	if exists {
		enabled, exists := config.GetEngineToggles()[voice.Engine][voice.Model]
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
		for _, enabled := range config.GetEngineToggles()[engine] {
			if enabled {
				engines = append(engines, engine)
				break
			}
		}
	}

	if len(engines) == 0 {
		issue.Trace(fmt.Errorf("No engines found"))
		return ""
	} else if len(engines) == 1 {
		return engines[0]
	} else {
		selectedEngine := engines[rand.Intn(len(engines)-1)]
		return selectedEngine
	}
}

func calculateVoice(engineID string, name string) (string, string, error) {
	if strings.Contains(name, ":") {
		segments := strings.Split(name, ":")

		if len(segments) < 2 {
			return "", "", issue.Trace(fmt.Errorf("Failed to parse voice name:" + name))
		}

		return segments[0], segments[1], nil
	} else {
		selectedEngine, _ := GetEngine(engineID)

		seed := int64(0)
		for _, r := range name {
			seed = seed*31 + int64(r)
		}
		rand.Seed(seed)

		modelToggles := config.GetEngineToggles()

		models := make([]string, 0, len(selectedEngine.Models))
		for modelID, _ := range selectedEngine.Models {
			if modelToggles[engineID][modelID] {
				models = append(models, modelID)
			}
		}

		var selectedModel string

		if len(models) == 0 {
			return "", "", issue.Trace(
				fmt.Errorf("No enabled models found for engine %s", selectedEngine),
			)
		} else if len(models) == 1 {
			selectedModel = models[0]
		} else {
			selectedModel = models[rand.Intn(len(models)-1)]
		}

		voices, _ := selectedEngine.Engine.GetVoices(selectedModel)
		if len(voices) == 0 {
			return "", "", issue.Trace(
				fmt.Errorf("No voices found for engine %s", selectedEngine),
			)
		}
		selectedVoice := voices[rand.Intn(len(voices)-1)]

		return selectedModel, selectedVoice.ID, nil
	}
}
