package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/config"
)

var httpClient = &http.Client{}

func (openAI *OpenAI) sendRequest(ctx context.Context, data OpenAIRequest) (io.ReadCloser, error) {
	apiKey := config.GetEngine().Api.OpenAI.ApiKey
	if apiKey == "" {
		return nil, response.Err("OpenAI API key is not set")
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, response.Err("Failed to marshal httpRequest body: %v", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/audio/speech", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, response.Err("Failed to create HTTP httpRequest: %v", err)
	}

	httpRequest.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		return nil, response.Err("Failed to send HTTP httpRequest: %v", err)
	}

	if httpResponse.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(httpResponse.Body)
		httpResponse.Body.Close()
		return nil, response.Err("HttpRequest failed with status %d: %s", httpResponse.StatusCode, string(bodyBytes))
	}

	response.Debug(util.MessageData{
		Summary: "OpenAI request succeeded",
		Detail:  "Response Status: " + httpResponse.Status,
	})

	return httpResponse.Body, nil
}
