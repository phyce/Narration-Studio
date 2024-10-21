package elevenlabs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
)

func getApiKey() string {
	apiKeyPointer := config.GetInstance().GetSetting("elevenlabsApiKey").String
	if apiKeyPointer == nil || *apiKeyPointer == "" {
		response.Debug(response.Data{
			Summary: "elevenlabsApiKey is empty",
		})
		return ""
	}
	return *apiKeyPointer
}

func FetchModels() (map[string]engine.Model, error) {
	apiKey := getApiKey()
	if apiKey == "" {
		return make(map[string]engine.Model, 0), util.TraceError(fmt.Errorf("api key is empty"))
	}

	client := &http.Client{}
	defer client.CloseIdleConnections()

	request, err := http.NewRequest("GET", "https://api.elevenlabs.io/v1/models", nil)
	if err != nil {
		return make(map[string]engine.Model, 0), util.TraceError(err)
	}
	request.Header.Set("xi-api-key", apiKey)

	response, err := client.Do(request)
	if err != nil {
		log.Fatalf("Failed to perform request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(response.Body)
		return make(map[string]engine.Model, 0), util.TraceError(errors.New(string(bodyBytes)))
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return make(map[string]engine.Model, 0), util.TraceError(err)
	}

	var modelsResponse []ModelResponse
	err = json.Unmarshal(bodyBytes, &modelsResponse)
	if err != nil {
		return make(map[string]engine.Model, 0), util.TraceError(err)
	}

	modelsMap := make(map[string]engine.Model)
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
	apiKey := getApiKey()
	if apiKey == "" {
		return make([]engine.Voice, 0), util.TraceError(fmt.Errorf("api key is empty"))
	}

	client := &http.Client{}
	defer client.CloseIdleConnections()

	request, err := http.NewRequest("GET", "https://api.elevenlabs.io/v1/voices", nil)
	if err != nil {
		return make([]engine.Voice, 0), util.TraceError(fmt.Errorf("creating request failed: %w", err))
	}

	request.Header.Set("xi-api-key", apiKey)

	response, err := client.Do(request)
	if err != nil {
		return make([]engine.Voice, 0), util.TraceError(fmt.Errorf("performing request failed: %w", err))
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(response.Body)
		return make([]engine.Voice, 0), util.TraceError(fmt.Errorf("unexpected status code: %d, response: %s", response.StatusCode, string(bodyBytes)))
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return make([]engine.Voice, 0), util.TraceError(fmt.Errorf("reading response body failed: %w", err))
	}

	var voicesResp VoicesResponse
	err = json.Unmarshal(bodyBytes, &voicesResp)
	if err != nil {
		return make([]engine.Voice, 0), util.TraceError(fmt.Errorf("parsing JSON failed: %w", err))
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
