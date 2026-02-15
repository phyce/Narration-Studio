package gemini

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nstudio/app/common/response"
	"nstudio/app/config"
	"time"
)

func (gemini *Gemini) sendRequest(request GeminiRequest, modelName string) ([]byte, error) {
	apiKey := config.GetEngine().Api.Gemini.ApiKey
	if apiKey == "" {
		return nil, fmt.Errorf("Gemini API key is not configured")
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, response.Err(err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", modelName, apiKey)

	httpRequest, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, response.Err(err)
	}

	httpRequest.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return nil, response.Err(err)
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(httpResponse.Body)
		return nil, fmt.Errorf("Gemini API error: %s - %s", httpResponse.Status, string(bodyBytes))
	}

	var geminiResponse GeminiResponse
	if err := json.NewDecoder(httpResponse.Body).Decode(&geminiResponse); err != nil {
		return nil, response.Err(err)
	}

	if len(geminiResponse.Candidates) == 0 || len(geminiResponse.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content in Gemini response")
	}

	base64Data := geminiResponse.Candidates[0].Content.Parts[0].InlineData.Data
	if base64Data == "" {
		return nil, fmt.Errorf("no audio data in Gemini response")
	}

	audioData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, response.Err(err)
	}

	return audioData, nil
}
