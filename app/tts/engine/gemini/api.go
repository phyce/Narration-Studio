package gemini

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"nstudio/app/common/response"
	"nstudio/app/config"
	"nstudio/app/tts/engine/internal/httpapi"
)

func (gemini *Gemini) sendRequest(request GeminiRequest, modelName string) ([]byte, error) {
	apiKey := config.GetEngine().Api.Gemini.ApiKey
	if apiKey == "" {
		return nil, fmt.Errorf("Gemini API key is not configured")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", modelName)

	// Send the API key via the documented header instead of a URL query param
	// so the secret does not leak into logs, proxies, or error bodies.
	body, err := httpapi.Do(httpapi.Request{
		Method: "POST",
		URL:    url,
		Headers: map[string]string{
			"x-goog-api-key": apiKey,
			"Content-Type":   "application/json",
		},
		Body: request,
	})
	if err != nil {
		return nil, err
	}

	var geminiResponse GeminiResponse
	if err := json.Unmarshal(body, &geminiResponse); err != nil {
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
