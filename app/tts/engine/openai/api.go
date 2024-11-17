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
		return nil, issue.Trace(fmt.Errorf("failed to marshal request body: %v", err))
	}

	request, err := http.NewRequest("POST", "https://api.openai.com/v1/audio/speech", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, issue.Trace(fmt.Errorf("failed to create HTTP request: %v", err))
	}

	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	defer client.CloseIdleConnections()

	httpResponse, err := client.Do(request)
	if err != nil {
		return nil, issue.Trace(fmt.Errorf("failed to send HTTP request: %v", err))
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(httpResponse.Body)
		return nil, issue.Trace(fmt.Errorf("request failed with status %d: %s", httpResponse.StatusCode, string(bodyBytes)))
	}

	responseData, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, issue.Trace(fmt.Errorf("failed to read response body: %v", err))
	}

	response.Success(response.Data{
		Summary: "Request succeeded?",
		Detail:  "Response Status: " + httpResponse.Status,
	})

	return responseData, nil
}
