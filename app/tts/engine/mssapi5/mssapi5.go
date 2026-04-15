package mssapi5

import (
	"encoding/json"
	"fmt"
	"nstudio/app/common/audio"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/common/util/fileIndex"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"os"
	"sync"
)

type MsSapi5 struct {
	voiceCache []engine.Voice
	mu         sync.RWMutex
}

func (sapi *MsSapi5) Initialize() error {
	return initCOM()
}

func (sapi *MsSapi5) Start(modelName string) error {
	return nil
}

func (sapi *MsSapi5) Stop(modelName string) error {
	return nil
}

func (sapi *MsSapi5) Play(message util.CharacterMessage) error {
	response.Debug(util.MessageData{
		Summary: "MS SAPI5 playing:" + message.Character,
		Detail:  message.Text,
	})

	payload := MsSapi5Request{
		Text:  message.Text,
		Voice: message.Voice.Voice,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return response.Err(err)
	}

	audioClip, err := sapi.Generate(message.Voice.Model, jsonPayload)
	if err != nil {
		return response.Err(err)
	}

	audio.PlayRawAudioBytes(audioClip)
	return nil
}

func (sapi *MsSapi5) Save(messages []util.CharacterMessage, play bool) error {
	response.Debug(util.MessageData{
		Summary: "MS SAPI5 saving messages",
	})

	err, outputPath := util.ExpandPath(config.GetSettings().OutputPath)
	if err != nil {
		return response.Err(err)
	}

	if err := os.MkdirAll(outputPath, os.ModePerm); err != nil {
		return response.Err(fmt.Errorf("failed to create output directory: %w", err))
	}

	for _, message := range messages {
		outputFilename := util.GenerateFilename(
			message,
			fileIndex.Get(),
			outputPath,
		)

		payload := MsSapi5Request{
			Text:  message.Text,
			Voice: message.Voice.Voice,
		}
		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			return response.Err(err)
		}

		audioClip, err := sapi.Generate(message.Voice.Model, jsonPayload)
		if err != nil {
			return response.Err(err)
		}

		err = os.WriteFile(outputFilename, audioClip, 0644)
		if err != nil {
			return response.Err(fmt.Errorf("failed to write audio to file '%s': %w", outputFilename, err))
		}

		if play {
			audio.PlayRawAudioBytes(audioClip)
		}
	}

	return nil
}

func (sapi *MsSapi5) Generate(model string, payload []byte) ([]byte, error) {
	var ttsPayload MsSapi5Request
	if err := json.Unmarshal(payload, &ttsPayload); err != nil {
		return nil, response.Err(fmt.Errorf("failed to unmarshal payload: %w", err))
	}

	if ttsPayload.Voice == "" {
		return nil, response.Err(fmt.Errorf("voice field is required in payload"))
	}

	if ttsPayload.Text == "" {
		return nil, response.Err(fmt.Errorf("text field is required in payload"))
	}

	cfg := config.GetEngine().Local.MsSapi5
	wavBytes, err := synthesize(ttsPayload.Voice, ttsPayload.Text, cfg.Rate, cfg.Volume)
	if err != nil {
		return nil, response.Err(err)
	}

	return wavBytes, nil
}

func (sapi *MsSapi5) GenerateAudio(model string, payload []byte) (*audio.Audio, error) {
	wavBytes, err := sapi.Generate(model, payload)
	if err != nil {
		return nil, err
	}

	return audio.NewAudioFromWAV(wavBytes)
}

func (sapi *MsSapi5) GetVoices(model string) ([]engine.Voice, error) {
	sapi.mu.RLock()
	if sapi.voiceCache != nil {
		defer sapi.mu.RUnlock()
		return sapi.voiceCache, nil
	}
	sapi.mu.RUnlock()

	sapi.mu.Lock()
	defer sapi.mu.Unlock()

	if sapi.voiceCache != nil {
		return sapi.voiceCache, nil
	}

	voices, err := enumerateVoices()
	if err != nil {
		return nil, response.Err(err)
	}

	sapi.voiceCache = voices
	return sapi.voiceCache, nil
}

func (sapi *MsSapi5) FetchModels() map[string]engine.Model {
	return FetchModels()
}

func FetchModels() map[string]engine.Model {
	return map[string]engine.Model{
		"mssapi5": {
			ID:     "mssapi5",
			Name:   "MS Speech API 5",
			Engine: "mssapi5",
		},
	}
}
