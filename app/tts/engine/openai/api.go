package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/config"
)

func getApiKey() string {
	apiKeyPointer := config.GetSetting("openAiApiKey").String
	if apiKeyPointer == nil || *apiKeyPointer == "" {
		response.Debug(response.Data{
			Summary: "openAiApiKey is empty",
		})
		return ""
	}
	return *apiKeyPointer
}

func (openAI *OpenAI) sendRequest(data OpenAIRequest) ([]byte, error) {

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, util.TraceError(fmt.Errorf("failed to marshal request body: %v", err))
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/audio/speech", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, util.TraceError(fmt.Errorf("failed to create HTTP request: %v", err))
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", getApiKey()))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	defer client.CloseIdleConnections()

	resp, err := client.Do(req)
	if err != nil {
		return nil, util.TraceError(fmt.Errorf("failed to send HTTP request: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, util.TraceError(fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes)))
	}

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, util.TraceError(fmt.Errorf("failed to read response body: %v", err))
	}

	response.Success(response.Data{
		Summary: "Request succeeded?",
		Detail:  "Response Status: " + resp.Status,
	})

	return responseData, nil
}
