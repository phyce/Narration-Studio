package http

import (
	"net/http"
	"nstudio/app/server/http/responses"
	"time"

	"nstudio/app/config"
	"nstudio/app/tts/modelManager"

	"github.com/labstack/echo/v4"
)

var serverStartTime = time.Now()

func handleHealth(context echo.Context) error {
	engines := modelManager.GetAllEngines()

	toggles := config.GetEngineToggles()

	var enabledEngines []responses.EngineHealthInfo
	totalModels := 0
	totalVoices := 0

	for _, engine := range engines {
		var models []responses.ModelInfo
		enabledCount := 0
		engineVoiceCount := 0
		totalEngineInstances := 0

		for _, model := range engine.Models {
			enabled := false
			if engineToggles, exists := toggles[engine.ID]; exists {
				if modelEnabled, modelExists := engineToggles[model.ID]; modelExists {
					enabled = modelEnabled
					if enabled {
						enabledCount++

						voices, err := modelManager.GetModelVoices(engine.ID, model.ID)
						if err == nil {
							engineVoiceCount += len(voices)
						}
					}
				}
			}

			modelInstanceCount := modelManager.GetInstanceCount(engine.ID, model.ID)
			if modelInstanceCount == 0 {
				modelInstanceCount = 1
			}
			totalEngineInstances += modelInstanceCount

			models = append(models, responses.ModelInfo{
				ID:        model.ID,
				Name:      model.Name,
				Enabled:   enabled,
				Instances: modelInstanceCount,
			})
		}

		totalModels += len(engine.Models)
		totalVoices += engineVoiceCount

		engineHealthInfo := responses.EngineHealthInfo{
			ID:           engine.ID,
			Name:         engine.Name,
			EnabledCount: enabledCount,
			TotalCount:   len(engine.Models),
			VoiceCount:   engineVoiceCount,
			Instances:    totalEngineInstances,
			Models:       models,
		}

		enabledEngines = append(enabledEngines, engineHealthInfo)
	}

	healthResponse := responses.HealthResponse{
		Status:         "healthy",
		Version:        config.GetInfo().Version,
		Uptime:         time.Since(serverStartTime).String(),
		EnabledEngines: enabledEngines,
		TotalModels:    totalModels,
		TotalVoices:    totalVoices,
	}

	return context.JSON(http.StatusOK, healthResponse)
}

func handleInfo(context echo.Context) error {
	return context.JSON(http.StatusOK, map[string]interface{}{
		"name":        "Narration Studio API",
		"version":     config.GetInfo().Version,
		"description": "Text-to-Speech API Server",
		"endpoints": map[string]interface{}{
			"health":              "/health",
			"info":                "/info",
			"profile-tts":         "/tts",
			"simple-tts":          "/tts/:engineId/:modelId/:voiceId",
			"engines":             "/engines",
			"engine-models":       "/engines/:engineId/models",
			"engine-model-voices": "/engines/:engineId/models/:modelId/voices",
			"voices":              "/voices",
			"profiles": map[string]string{
				"list":   "/profiles",
				"get":    "/profiles/:profileId",
				"create": "/profiles",
				"delete": "/profiles/:profileId",
				"voices": "/profiles/:profileId/voices",
			},
		},
	})
}
