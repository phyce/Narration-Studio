package openai

import (
	"fmt"
	"nstudio/app/common/response"
	"nstudio/app/config"
	"nstudio/app/tts/engine/internal/httpapi"
)

func (openAI *OpenAI) sendRequest(data OpenAIRequest) ([]byte, error) {
	apiKey := config.GetEngine().Api.OpenAI.ApiKey
	if apiKey == "" {
		return nil, response.Err(fmt.Errorf("OpenAI API key is not set"))
	}

	return httpapi.Do(httpapi.Request{
		Method: "POST",
		URL:    "https://api.openai.com/v1/audio/speech",
		Headers: map[string]string{
			"Authorization": "Bearer " + apiKey,
			"Content-Type":  "application/json",
		},
		Body: data,
	})
}
