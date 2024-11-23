package elevenlabs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"nstudio/app/common/issue"
	"nstudio/app/common/response"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
)

func FetchModels() (map[string]engine.Model, error) {
	apiKey := config.GetEngine().Api.ElevenLabs.ApiKey
	if apiKey == "" {
		return make(map[string]engine.Model, 0), issue.Trace(fmt.Errorf("Api key is empty"))
	}

	modelsMap := make(map[string]engine.Model)

	client := &http.Client{}
	defer client.CloseIdleConnections()

	request, err := http.NewRequest("GET", "https://api.elevenlabs.io/v1/models", nil)
	if err != nil {
		return modelsMap, issue.Trace(err)
	}
	request.Header.Set("xi-api-key", apiKey)

	httpResponse, err := client.Do(request)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to fetch elevenlabs models",
			Detail:  err.Error(),
		})
		return modelsMap, issue.Trace(err)
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(httpResponse.Body)
		return make(map[string]engine.Model, 0), issue.Trace(errors.New(string(bodyBytes)))
	}

	bodyBytes, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return make(map[string]engine.Model, 0), issue.Trace(err)
	}

	var modelsResponse []ModelResponse
	err = json.Unmarshal(bodyBytes, &modelsResponse)
	if err != nil {
		return make(map[string]engine.Model, 0), issue.Trace(err)
	}

	for _, m := range modelsResponse {
		model := engine.Model{
			ID:     m.ModelID,
			Name:   m.Name,
			Engine: "elevenlabs",
		}
		modelsMap[m.ModelID] = model
	}

	return modelsMap, nil
}

func FetchVoices() ([]engine.Voice, error) {
	apiKey := config.GetEngine().Api.ElevenLabs.ApiKey
	if apiKey == "" {
		return make([]engine.Voice, 0), issue.Trace(fmt.Errorf("api key is empty"))
	}

	client := &http.Client{}
	defer client.CloseIdleConnections()

	request, err := http.NewRequest("GET", "https://api.elevenlabs.io/v1/voices", nil)
	if err != nil {
		return make([]engine.Voice, 0), issue.Trace(fmt.Errorf("creating request failed: %w", err))
	}

	request.Header.Set("xi-api-key", apiKey)

	response, err := client.Do(request)
	if err != nil {
		return make([]engine.Voice, 0), issue.Trace(fmt.Errorf("performing request failed: %w", err))
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(response.Body)
		return make([]engine.Voice, 0), issue.Trace(fmt.Errorf("unexpected status code: %d, response: %s", response.StatusCode, string(bodyBytes)))
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return make([]engine.Voice, 0), issue.Trace(fmt.Errorf("reading response body failed: %w", err))
	}

	var voicesResp VoicesResponse
	err = json.Unmarshal(bodyBytes, &voicesResp)
	if err != nil {
		return make([]engine.Voice, 0), issue.Trace(fmt.Errorf("parsing JSON failed: %w", err))
	}

	responseVoices := make([]engine.Voice, 0, len(voicesResp.Voices))
	for _, vd := range voicesResp.Voices {
		voice := engine.Voice{
			ID:     vd.VoiceID,
			Name:   vd.Name,
			Gender: vd.Labels.Gender,
		}
		responseVoices = append(responseVoices, voice)
	}
	return responseVoices, nil
}
