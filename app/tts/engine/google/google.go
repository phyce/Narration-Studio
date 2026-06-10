package google

import (
	"context"
	"encoding/json"
	"fmt"
	"nstudio/app/common/audio"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/common/util/fileIndex"
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

func (google *Google) Play(message util.CharacterMessage) error {
	response.Debug(util.MessageData{
		Summary: "Google playing:" + message.Character,
		Detail:  message.Text,
	})

	input := GoogleRequest{
		Input: Input{Text: message.Text},
		Voice: VoiceSelectionParams{
			Name:         message.Voice.Voice,
			LanguageCode: "en-US", // Default
			ModelName:    message.Voice.Model,
		},
		AudioConfig: AudioConfig{
			AudioEncoding: "MP3",
			SpeakingRate:  1,
			Pitch:         0,
		},
	}

	// Attempt to extract language code
	parts := strings.Split(message.Voice.Voice, "-")
	if len(parts) >= 2 {
		input.Voice.LanguageCode = parts[0] + "-" + parts[1]
	}

	audioClip, err := google.sendRequest(context.Background(), input)
	if err != nil {
		return response.Err("%w", err)
	}

	err = audio.PlayMP3AudioBytes(audioClip)
	if err != nil {
		return response.Err("%w", err)
	}

	return response.Success(util.MessageData{
		Summary: "Google finished playing mp3",
	})
}

func (google *Google) Save(messages []util.CharacterMessage, play bool) error {
	response.Debug(util.MessageData{
		Summary: "Google saving messages",
	})

	err, expandedPath := util.ExpandPath(config.GetSettings().OutputPath)
	if err != nil {
		return response.Err("%w", err)
	}

	for _, message := range messages {
		input := GoogleRequest{
			Input: Input{Text: message.Text},
			Voice: VoiceSelectionParams{
				Name:         message.Voice.Voice,
				LanguageCode: "en-US",
				ModelName:    message.Voice.Model,
			},
			AudioConfig: AudioConfig{
				AudioEncoding: "LINEAR16",
				SpeakingRate:  1,
				Pitch:         0,
			},
		}

		parts := strings.Split(message.Voice.Voice, "-")
		if len(parts) >= 2 {
			input.Voice.LanguageCode = parts[0] + "-" + parts[1]
		}

		audioClip, err := google.sendRequest(context.Background(), input)
		if err != nil {
			return response.Err("%w", err)
		}

		filename := util.GenerateFilename(
			message,
			fileIndex.Get(),
			expandedPath,
		)

		err = audio.SaveWAVFile(audioClip, filename)
		if err != nil {
			return response.Err("%w", err)
		}

		if play {
			err = audio.PlayPCMAudioBytes(audioClip)
			if err != nil {
				return response.Err("%w", err)
			}
		}
	}

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

func (google *Google) Generate(model string, payload []byte) ([]byte, error) {
	fmt.Println(string(payload))
	var request GoogleRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, response.Err("%w", err)
	}

	request.AudioConfig.AudioEncoding = "LINEAR16"

	pcmData, err := google.sendRequest(context.Background(), request)
	if err != nil {
		return nil, response.Err("%w", err)
	}

	return pcmData, nil
}

func (google *Google) GenerateAudio(model string, payload []byte) (*audio.Audio, error) {
	pcmData, err := google.Generate(model, payload)
	if err != nil {
		return nil, response.Err("%w", err)
	}

	return audio.NewAudioFromPCM(pcmData, 24000, 1, 16), nil
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
