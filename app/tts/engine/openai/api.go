package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nstudio/app/common/issue"
	"nstudio/app/common/response"
	"nstudio/app/config"
)

func (openAI *OpenAI) sendRequest(data OpenAIRequest) ([]byte, error) {
	apiKey := config.GetEngine().Api.OpenAI.ApiKey
	if apiKey == "" {
		return nil, issue.Trace(fmt.Errorf("OpenAI API key is not set"))
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, issue.Trace(fmt.Errorf("Failed to marshal httpRequest body: %v", err))
	}

	httpRequest, err := http.NewRequest("POST", "https://api.openai.com/v1/audio/speech", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, issue.Trace(fmt.Errorf("Failed to create HTTP httpRequest: %v", err))
	}

	httpRequest.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	httpRequest.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	defer client.CloseIdleConnections()

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return nil, issue.Trace(fmt.Errorf("Failed to send HTTP httpRequest: %v", err))
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(httpResponse.Body)
		return nil, issue.Trace(fmt.Errorf("HttpRequest failed with status %d: %s", httpResponse.StatusCode, string(bodyBytes)))
	}

	responseData, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, issue.Trace(fmt.Errorf("Failed to read response body: %v", err))
	}

	response.Success(response.Data{
		Summary: "Request succeeded?",
		Detail:  "Response Status: " + httpResponse.Status,
	})

	return responseData, nil
}
