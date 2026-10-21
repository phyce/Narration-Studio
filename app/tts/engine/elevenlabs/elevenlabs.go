package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	commonAudio "nstudio/app/common/audio"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
)

type ElevenLabs struct {
	Models     map[string]Model
	outputType string
}

var voices = make([]engine.Voice, 0)

var httpClient = &http.Client{}

// <editor-fold desc="Engine Interface">
func (labs *ElevenLabs) Initialize() error {
	var err error
	voices, err = FetchVoices()
	if err != nil {
		return response.Err("%w", err)
	}

	labs.outputType = "pcm_24000"

	//TODO add api key check

	return nil
}

func (labs *ElevenLabs) Start(modelName string) error {
	return nil
}

func (labs *ElevenLabs) Stop(modelName string) error {
	return nil
}

func (labs *ElevenLabs) Synthesize(ctx context.Context, request engine.SpeechRequest) (*commonAudio.Stream, error) {
	input := ElevenLabsRequest{
		Text:    request.Text,
		ModelID: request.Model,
		VoiceID: request.Voice,
		VoiceSettings: VoiceSettings{
			Stability:       0.5,
			SimilarityBoost: 0.5,
		},
	}

	body, err := labs.sendRequest(ctx, request.Voice, input)
	if err != nil {
		return nil, response.Err("%w", err)
	}

	format := commonAudio.StreamFormat{SampleRate: 24000, Channels: 1, BitDepth: 16}
	return commonAudio.NewProducedStream(format, func(writer *commonAudio.StreamWriter) error {
		defer body.Close()
		_, copyErr := io.Copy(writer, body)
		return copyErr
	}), nil
}

func (labs *ElevenLabs) GetVoices(model string) ([]engine.Voice, error) {
	return voices, nil
}

func (labs *ElevenLabs) FetchModels() map[string]engine.Model {
	apiKey := config.GetEngine().Api.ElevenLabs.ApiKey
	if apiKey == "" {
		return make(map[string]engine.Model)
	}

	models, err := FetchModels()
	if err != nil {
		response.Err("Failed to fetch elevenlabs models: %w", err)
		return make(map[string]engine.Model)
	}

	voices, err = FetchVoices()
	if err != nil {
		response.Err("Failed to fetch elevenlabs voices: %w", err)
		return make(map[string]engine.Model)
	}

	return models
}

// </editor-fold>

func (labs *ElevenLabs) CheckAvailability(ctx context.Context) error {
	apiKey := config.GetEngine().Api.ElevenLabs.ApiKey
	if apiKey == "" {
		return engine.ErrApiKeyNotSet
	}
	return engine.CheckApiAnswers(ctx, "ElevenLabs", "https://api.elevenlabs.io/v1/models", map[string]string{
		"xi-api-key": apiKey,
	})
}

// <editor-fold desc="Other">
func (labs *ElevenLabs) sendRequest(ctx context.Context, voiceID string, data ElevenLabsRequest) (io.ReadCloser, error) {
	apiKey := config.GetEngine().Api.ElevenLabs.ApiKey
	if apiKey == "" {
		return nil, response.Err("Elevenlabs API Key is not set")
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, response.Err("failed to marshal request body: %v", err)
	}

	url := fmt.Sprintf("https://api.elevenlabs.io/v1/text-to-speech/%s/stream?output_format=%s", voiceID, labs.outputType)

	httpRequest, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, response.Err("failed to create HTTP request: %v", err)
	}

	httpRequest.Header.Set("xi-api-key", apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		return nil, response.Err("failed to send HTTP request: %v", err)
	}

	if httpResponse.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(httpResponse.Body)
		httpResponse.Body.Close()
		return nil, response.Err("request failed with status %d: %s", httpResponse.StatusCode, string(bodyBytes))
	}

	response.Debug(util.MessageData{
		Summary: "ElevenLabs request succeeded",
		Detail:  "Response Status: " + httpResponse.Status,
	})

	return httpResponse.Body, nil
}

// </editor-fold>
