package google

import (
	"context"
	"nstudio/app/common/audio"
	"nstudio/app/common/response"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"strings"
	"sync"
)

type Google struct {
	Models     map[string]Model
	voiceCache map[string][]engine.Voice
	mu         sync.RWMutex
}

func (google *Google) Initialize() error {
	google.mu.Lock()
	defer google.mu.Unlock()
	google.voiceCache = make(map[string][]engine.Voice)
	//TODO add api key check
	return nil
}

func (google *Google) Start(modelName string) error {
	return nil
}

func (google *Google) Stop(modelName string) error {
	return nil
}

func (google *Google) Synthesize(ctx context.Context, request engine.SpeechRequest) (*audio.Stream, error) {
	input := GoogleRequest{
		Input: Input{Text: request.Text},
		Voice: VoiceSelectionParams{
			Name:         request.Voice,
			LanguageCode: languageCodeFromVoice(request.Voice),
			ModelName:    request.Model,
		},
		AudioConfig: AudioConfig{
			AudioEncoding: "LINEAR16",
			SpeakingRate:  1,
			Pitch:         0,
		},
	}

	audioContent, err := google.sendRequest(ctx, input)
	if err != nil {
		return nil, response.Err("%w", err)
	}

	// LINEAR16 responses carry a WAV header; honor its embedded format instead
	// of playing the header bytes as PCM at an assumed rate.
	if isWAV(audioContent) {
		wavAudio, err := audio.NewAudioFromWAV(audioContent)
		if err != nil {
			return nil, response.Err("%w", err)
		}
		return audio.NewStreamFromAudio(wavAudio)
	}

	return audio.NewBufferedStream(audio.StreamFormat{SampleRate: 24000, Channels: 1, BitDepth: 16}, audioContent), nil
}

func isWAV(data []byte) bool {
	return len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WAVE"
}

func languageCodeFromVoice(voice string) string {
	parts := strings.Split(voice, "-")
	if len(parts) >= 2 {
		return parts[0] + "-" + parts[1]
	}
	return "en-US"
}

func (google *Google) GetVoices(model string) ([]engine.Voice, error) {
	return google.fetchVoices(model)
}

func (google *Google) FetchModels() map[string]engine.Model {
	return FetchModels()
}

// </editor-fold>

// <editor-fold desc="Other">
func FetchModels() map[string]engine.Model {
	if config.GetEngine().Api.Google.ApiKey == "" {
		return make(map[string]engine.Model)
	}
	return map[string]engine.Model{
		"standard": {
			ID:     "standard",
			Name:   "Standard",
			Engine: "google",
		},
		"wavenet": {
			ID:     "wavenet",
			Name:   "WaveNet",
			Engine: "google",
		},
		"neural2": {
			ID:     "neural2",
			Name:   "Neural2",
			Engine: "google",
		},
		"polyglot": {
			ID:     "polyglot",
			Name:   "Polyglot",
			Engine: "google",
		},
		"studio": {
			ID:     "studio",
			Name:   "Studio",
			Engine: "google",
		},
		"chirp": {
			ID:     "chirp",
			Name:   "Chirp",
			Engine: "google",
		},
		//"chirp-3": {
		//	ID:     "chirp-3",
		//	Name:   "Chirp 3",
		//	Engine: "google",
		//},
		"chirp-3-hd": {
			ID:     "chirp-3-hd",
			Name:   "Chirp 3 HD",
			Engine: "google",
		},
		"gemini-tts": {
			ID:     "gemini-tts",
			Name:   "Gemini TTS",
			Engine: "google",
		},
	}
}

type Model struct {
	Voices []engine.Voice `json:"voice"`
}
