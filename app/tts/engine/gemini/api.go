package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nstudio/app/common/response"
	"nstudio/app/config"
	"strings"
	"time"
)

func (gemini *Gemini) sendRequest(ctx context.Context, request GeminiRequest, modelName string) ([]byte, error) {
	apiKey := config.GetEngine().Api.Gemini.ApiKey
	if apiKey == "" {
		return nil, response.Err("Gemini API key is not configured")
	}

	for contentIndex := range request.Contents {
		for partIndex := range request.Contents[contentIndex].Parts {
			text := request.Contents[contentIndex].Parts[partIndex].Text
			if text != "" && !strings.HasPrefix(text, "TTS: ") {
				request.Contents[contentIndex].Parts[partIndex].Text = "TTS: " + text
			}
		}
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, response.Err("%w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", modelName, apiKey)

	httpRequest, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, response.Err("%w", err)
	}

	httpRequest.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return nil, response.Err("%w", err)
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(httpResponse.Body)
		return nil, response.Err("Gemini API error: %s - %s", httpResponse.Status, string(bodyBytes))
	}

	var geminiResponse GeminiResponse
	if err := json.NewDecoder(httpResponse.Body).Decode(&geminiResponse); err != nil {
		return nil, response.Err("%w", err)
	}

	if len(geminiResponse.Candidates) == 0 || len(geminiResponse.Candidates[0].Content.Parts) == 0 {
		return nil, response.Err("no content in Gemini response")
	}

	base64Data := geminiResponse.Candidates[0].Content.Parts[0].InlineData.Data
	if base64Data == "" {
		return nil, response.Err("no audio data in Gemini response")
	}

	audioData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, response.Err("%w", err)
	}

	return audioData, nil
}
