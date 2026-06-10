package mssapi5

import (
	"context"
	"nstudio/app/common/audio"
	"nstudio/app/common/response"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
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

func (sapi *MsSapi5) Synthesize(ctx context.Context, request engine.SpeechRequest) (*audio.Stream, error) {
	if request.Voice == "" {
		return nil, response.Err("voice field is required")
	}
	if request.Text == "" {
		return nil, response.Err("text field is required")
	}

	settings := config.GetEngine().Local.MsSapi5
	wavBytes, err := synthesize(request.Voice, request.Text, settings.Rate, settings.Volume)
	if err != nil {
		return nil, response.Err("%w", err)
	}

	wavAudio, err := audio.NewAudioFromWAV(wavBytes)
	if err != nil {
		return nil, response.Err("%w", err)
	}

	return audio.NewStreamFromAudio(wavAudio)
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
		return nil, response.Err("%w", err)
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
