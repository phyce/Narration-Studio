package engines

import (
	"fmt"
	"net/http"
	"nstudio/app/server/http/responses"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/modelManager"

	"github.com/labstack/echo/v4"
)

func GetEngines(context echo.Context) error {
	engines := modelManager.GetEngines()
	engineList := make([]map[string]interface{}, 0, len(engines))

	for _, engine := range engines {
		engineList = append(engineList, map[string]interface{}{
			"id":     engine.ID,
			"name":   engine.Name,
			"models": len(engine.Models),
		})
	}

	return context.JSON(http.StatusOK, map[string]interface{}{
		"engines": engineList,
		"count":   len(engines),
	})
}

func GetModels(context echo.Context) error {
	engineId := context.Param("engineId")

	if engineId == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Engine ID is required",
			Code:    400,
		})
	}

	engines := modelManager.GetEngines()

	var targetEngine *engine.Engine
	for _, eng := range engines {
		if eng.ID == engineId {
			targetEngine = &eng
			break
		}
	}

	if targetEngine == nil {
		return context.JSON(http.StatusNotFound, responses.ErrorResponse{
			Success: false,
			Error:   fmt.Sprintf("Engine not found: %s", engineId),
			Code:    404,
		})
	}

	modelList := make([]map[string]interface{}, 0, len(targetEngine.Models))
	for _, model := range targetEngine.Models {
		modelList = append(modelList, map[string]interface{}{
			"id":     model.ID,
			"name":   model.Name,
			"engine": model.Engine,
		})
	}

	return context.JSON(http.StatusOK, map[string]interface{}{
		"engine": engineId,
		"models": modelList,
		"count":  len(modelList),
	})
}

func GetVoices(context echo.Context) error {
	engineId := context.Param("engineId")
	modelId := context.Param("modelId")

	if engineId == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Engine ID is required",
			Code:    400,
		})
	}

	if modelId == "" {
		return context.JSON(http.StatusBadRequest, responses.ErrorResponse{
			Success: false,
			Error:   "Model ID is required",
			Code:    400,
		})
	}

	voices, err := modelManager.GetModelVoices(engineId, modelId)
	if err != nil {
		return context.JSON(http.StatusNotFound, responses.ErrorResponse{
			Success: false,
			Error:   fmt.Sprintf("Model not found or no voices available: %s/%s", engineId, modelId),
			Code:    404,
		})
	}

	return context.JSON(http.StatusOK, voices)
}

func GetAllVoices(context echo.Context) error {
	engines := modelManager.GetEngines()

	type ModelWithVoices struct {
		ID     string         `json:"id"`
		Name   string         `json:"name"`
		Voices []engine.Voice `json:"voices"`
	}

	type EngineWithModels struct {
		ID     string            `json:"id"`
		Name   string            `json:"name"`
		Models []ModelWithVoices `json:"models"`
	}

	engineList := make([]EngineWithModels, 0, len(engines))

	for _, eng := range engines {
		modelList := make([]ModelWithVoices, 0, len(eng.Models))

		for _, model := range eng.Models {
			voices, err := modelManager.GetModelVoices(eng.ID, model.ID)
			if err != nil {
				voices = []engine.Voice{}
			}

			modelList = append(modelList, ModelWithVoices{
				ID:     model.ID,
				Name:   model.Name,
				Voices: voices,
			})
		}

		engineList = append(engineList, EngineWithModels{
			ID:     eng.ID,
			Name:   eng.Name,
			Models: modelList,
		})
	}

	return context.JSON(http.StatusOK, engineList)
}
