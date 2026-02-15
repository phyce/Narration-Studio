package openai

import (
	"encoding/json"
	"nstudio/app/common/audio"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/common/util/fileIndex"
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
	//openAI.outputType = defaults.GetEngine().Api.OpenAI.OutputType
	openAI.outputType = "flac"

	//TODO add api key check

	return nil
}

func (openAI *OpenAI) Start(modelName string) error {
	return nil
}
func (openAI *OpenAI) Stop(modelName string) error {
	return nil
}

func (openAI *OpenAI) Play(message util.CharacterMessage) error {
	response.Debug(util.MessageData{
		Summary: "OpenAI playing:" + message.Character,
		Detail:  message.Text,
	})

	input := OpenAIRequest{
		Voice:          message.Voice.Voice,
		Input:          message.Text,
		Model:          message.Voice.Model,
		ResponseFormat: openAI.outputType,
		Speed:          1,
	}

	audioClip, err := openAI.sendRequest(input)
	if err != nil {
		return response.Err(err)
	}

	err = audio.PlayFLACAudioBytes(audioClip)
	if err != nil {
		return response.Err(err)
	}

	return response.Success(util.MessageData{
		Summary: "OpenAI finished playing flac",
	})
}

func (openAI *OpenAI) Save(messages []util.CharacterMessage, play bool) error {
	response.Debug(util.MessageData{
		Summary: "Openai saving messages",
	})

	err, expandedPath := util.ExpandPath(config.GetSettings().OutputPath)
	if err != nil {
		return response.Err(err)
	}

	for _, message := range messages {
		input := OpenAIRequest{
			Voice:          message.Voice.Voice,
			Input:          message.Text,
			Model:          message.Voice.Model,
			ResponseFormat: openAI.outputType,
			Speed:          1,
		}

		audioClip, err := openAI.sendRequest(input)
		if err != nil {
			return response.Err(err)
		}

		filename := util.GenerateFilename(
			message,
			fileIndex.Get(),
			expandedPath,
		)

		err = audio.SaveFLACAsWAV(audioClip, filename)
		if err != nil {
			return response.Err(err)
		}

		if play {
			err = audio.PlayFLACAudioBytes(audioClip)
			if err != nil {
				return response.Err(err)
			}
		}
	}

	return nil
}

func (openAI *OpenAI) Generate(model string, payload []byte) ([]byte, error) {
	var request OpenAIRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, response.Err(err)
	}

	request.Model = model
	request.ResponseFormat = openAI.outputType
	request.Speed = 1

	flacData, err := openAI.sendRequest(request)
	if err != nil {
		return nil, response.Err(err)
	}

	return flacData, nil
}

func (openAI *OpenAI) GenerateAudio(model string, payload []byte) (*audio.Audio, error) {
	flacData, err := openAI.Generate(model, payload)
	if err != nil {
		return nil, err
	}

	return audio.NewAudioFromFLAC(flacData), nil
}

func (openAI *OpenAI) GetVoices(model string) ([]engine.Voice, error) {
	return voices, nil
}

func (openAI *OpenAI) FetchModels() map[string]engine.Model {
	apiKey := config.GetEngine().Api.OpenAI.ApiKey
	if apiKey == "" {
		return make(map[string]engine.Model)
	}

	return FetchModels()
}

// </editor-fold>

// <editor-fold desc="Other">
func FetchModels() map[string]engine.Model {
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
