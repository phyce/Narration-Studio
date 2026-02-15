package elevenlabs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	commonAudio "nstudio/app/common/audio"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/common/util/fileIndex"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
)

type ElevenLabs struct {
	Models     map[string]Model
	outputType string
}

var voices = make([]engine.Voice, 0)

// <editor-fold desc="Engine Interface">
func (labs *ElevenLabs) Initialize() error {
	var err error
	voices, err = FetchVoices()
	if err != nil {
		return response.Err(err)
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

func (labs *ElevenLabs) Play(message util.CharacterMessage) error {
	response.Debug(util.MessageData{
		Summary: "Elevenlabs playing:" + message.Character,
		Detail:  message.Text,
	})

	input := ElevenLabsRequest{
		Text:    message.Text,
		ModelID: message.Voice.Model,
		VoiceSettings: VoiceSettings{
			Stability:       0.5,
			SimilarityBoost: 0.5,
		},
	}

	audioClip, err := labs.sendRequest(message.Voice.Voice, input)
	if err != nil {
		return response.Err(err)
	}

	err = commonAudio.PlayPCMAudioBytes(audioClip)
	if err != nil {
		return response.Err(err)
	}

	return response.Success(util.MessageData{
		Summary: "ElevenLabs finished playing audio",
	})

	return nil
}

func (labs *ElevenLabs) Save(messages []util.CharacterMessage, play bool) error {
	response.Debug(util.MessageData{
		Summary: "Elevenlabs saving messages",
	})

	err, expandedPath := util.ExpandPath(config.GetSettings().OutputPath)
	if err != nil {
		return response.Err(err)
	}

	for _, message := range messages {
		input := ElevenLabsRequest{
			Text:    message.Text,
			ModelID: message.Voice.Model,
			VoiceSettings: VoiceSettings{
				Stability:       0.5,
				SimilarityBoost: 0.5,
			},
		}

		audioClip, err := labs.sendRequest(message.Voice.Voice, input)
		if err != nil {
			return response.Err(err)
		}

		filename := util.GenerateFilename(
			message,
			fileIndex.Get(),
			expandedPath,
		)

		err = commonAudio.SaveWAVFile(audioClip, filename)
		if err != nil {
			return response.Err(err)
		}

		if play {
			err = commonAudio.PlayPCMAudioBytes(audioClip)
			if err != nil {
				return response.Err(err)
			}
		}
	}

	return nil
}

func (labs *ElevenLabs) Generate(model string, payload []byte) ([]byte, error) {
	var request ElevenLabsRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, response.Err(err)
	}

	pcmData, err := labs.sendRequest(model, request)
	if err != nil {
		return nil, response.Err(err)
	}

	return pcmData, nil
}

func (labs *ElevenLabs) GenerateAudio(model string, payload []byte) (*commonAudio.Audio, error) {
	pcmData, err := labs.Generate(model, payload)
	if err != nil {
		return nil, err
	}

	return commonAudio.NewAudioFromPCM(pcmData, 24000, 1, 16), nil
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
		response.Error(util.MessageData{
			Summary: "Failed to fetch elevenlabs models",
			Detail:  err.Error(),
		})
		return make(map[string]engine.Model)
	}

	voices, err = FetchVoices()
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to fetch elevenlabs voices",
			Detail:  err.Error(),
		})
		return make(map[string]engine.Model)
	}

	return models
}

// </editor-fold>

// <editor-fold desc="Other">
func (labs *ElevenLabs) sendRequest(voiceID string, data ElevenLabsRequest) ([]byte, error) {
	apiKey := config.GetEngine().Api.ElevenLabs.ApiKey
	if apiKey == "" {
		return nil, response.Err(fmt.Errorf("Elevenlabs API Key is not set"))
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, response.Err(fmt.Errorf("failed to marshal request body: %v", err))
	}

	url := fmt.Sprintf("https://api.elevenlabs.io/v1/text-to-speech/%s?output_format=%s", voiceID, labs.outputType)

	httpRequest, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, response.Err(fmt.Errorf("failed to create HTTP request: %v", err))
	}

	httpRequest.Header.Set("xi-api-key", apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	defer client.CloseIdleConnections()

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return nil, response.Err(fmt.Errorf("failed to send HTTP request: %v", err))
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(httpResponse.Body)
		return nil, response.Err(fmt.Errorf("request failed with status %d: %s", httpResponse.StatusCode, string(bodyBytes)))
	}

	responseData, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, response.Err(fmt.Errorf("failed to read response body: %v", err))
	}

	response.Success(util.MessageData{
		Summary: "ElevenLabs request succeeded",
		Detail:  "Response Status: " + httpResponse.Status,
	})

	return responseData, nil
}

// </editor-fold>
