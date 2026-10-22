package openai

import (
	"context"
	"io"
	"nstudio/app/common/audio"
	"nstudio/app/common/response"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
)

type OpenAI struct {
	Models     map[string]Model
	outputType string
}

var voices = []engine.Voice{
	{ID: "alloy", Name: "Alloy", Gender: ""},
	{ID: "echo", Name: "Echo", Gender: ""},
	{ID: "fable", Name: "Fable", Gender: ""},
	{ID: "onyx", Name: "Onyx", Gender: ""},
	{ID: "nova", Name: "Nova", Gender: ""},
	{ID: "shimmer", Name: "Shimmer", Gender: ""},
}

// <editor-fold desc="Engine Interface">
func (openAI *OpenAI) Initialize() error {
	openAI.outputType = "pcm"

	//TODO add api key check

	return nil
}

func (openAI *OpenAI) Start(modelName string) error {
	return nil
}
func (openAI *OpenAI) Stop(modelName string) error {
	return nil
}

func (openAI *OpenAI) Synthesize(ctx context.Context, request engine.SpeechRequest) (*audio.Stream, error) {
	input := OpenAIRequest{
		Voice:          request.Voice,
		Input:          request.Text,
		Model:          request.Model,
		ResponseFormat: openAI.outputType,
		Speed:          1,
	}

	body, err := openAI.sendRequest(ctx, input)
	if err != nil {
		return nil, response.Err("%w", err)
	}

	format := audio.StreamFormat{SampleRate: 24000, Channels: 1, BitDepth: 16}
	return audio.NewProducedStream(format, func(writer *audio.StreamWriter) error {
		defer body.Close()
		_, copyErr := io.Copy(writer, body)
		return copyErr
	}), nil
}

func (openAI *OpenAI) GetVoices(model string) ([]engine.Voice, error) {
	return voices, nil
}

func (openAI *OpenAI) FetchModels() map[string]engine.Model {
	return FetchModels()
}

// </editor-fold>

func (openAI *OpenAI) CheckAvailability(ctx context.Context) error {
	apiKey := config.GetEngine().Api.OpenAI.ApiKey
	if apiKey == "" {
		return engine.ErrApiKeyNotSet
	}
	return engine.CheckApiAnswers(ctx, "OpenAI", "https://api.openai.com/v1/models", map[string]string{
		"Authorization": "Bearer " + apiKey,
	})
}

// <editor-fold desc="Other">
func FetchModels() map[string]engine.Model {
	if config.GetEngine().Api.OpenAI.ApiKey == "" {
		return make(map[string]engine.Model)
	}
	return map[string]engine.Model{
		"tts-1": {
			ID:     "tts-1",
			Name:   "TTS-1",
			Engine: "openai",
		},
		"tts-1-hd": {
			ID:     "tts-1-hd",
			Name:   "TTS-1 HD",
			Engine: "openai",
		},
	}
}

// </editor-fold>
