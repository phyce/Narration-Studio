package elevenlabs

import (
	"encoding/json"
	"fmt"
	"nstudio/app/common/response"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/engine/internal/httpapi"
)

func FetchModels() (map[string]engine.Model, error) {
	apiKey := config.GetEngine().Api.ElevenLabs.ApiKey
	if apiKey == "" {
		return make(map[string]engine.Model, 0), response.Err(fmt.Errorf("Api key is empty"))
	}

	body, err := httpapi.Do(httpapi.Request{
		Method:  "GET",
		URL:     "https://api.elevenlabs.io/v1/models",
		Headers: map[string]string{"xi-api-key": apiKey},
	})
	if err != nil {
		return make(map[string]engine.Model, 0), err
	}

	var modelsResponse []ModelResponse
	if err := json.Unmarshal(body, &modelsResponse); err != nil {
		return make(map[string]engine.Model, 0), response.Err(err)
	}

	modelsMap := make(map[string]engine.Model)
	for _, m := range modelsResponse {
		modelsMap[m.ModelID] = engine.Model{
			ID:     m.ModelID,
			Name:   m.Name,
			Engine: "elevenlabs",
		}
	}

	return modelsMap, nil
}

func FetchVoices() ([]engine.Voice, error) {
	apiKey := config.GetEngine().Api.ElevenLabs.ApiKey
	if apiKey == "" {
		return make([]engine.Voice, 0), response.Err(fmt.Errorf("api key is empty"))
	}

	body, err := httpapi.Do(httpapi.Request{
		Method:  "GET",
		URL:     "https://api.elevenlabs.io/v1/voices",
		Headers: map[string]string{"xi-api-key": apiKey},
	})
	if err != nil {
		return make([]engine.Voice, 0), err
	}

	var voicesResp VoicesResponse
	if err := json.Unmarshal(body, &voicesResp); err != nil {
		return make([]engine.Voice, 0), response.Err(fmt.Errorf("parsing JSON failed: %w", err))
	}

	responseVoices := make([]engine.Voice, 0, len(voicesResp.Voices))
	for _, vd := range voicesResp.Voices {
		responseVoices = append(responseVoices, engine.Voice{
			ID:     vd.VoiceID,
			Name:   vd.Name,
			Gender: vd.Labels.Gender,
		})
	}

	return responseVoices, nil
}
