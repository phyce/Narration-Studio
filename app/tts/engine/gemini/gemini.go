package gemini

import (
	"encoding/json"
	"nstudio/app/common/audio"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/common/util/fileIndex"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"sync"
)

type Gemini struct {
	Models     map[string]Model
	voiceCache map[string][]engine.Voice
	mu         sync.RWMutex
}

func (gemini *Gemini) Initialize() error {
	gemini.mu.Lock()
	defer gemini.mu.Unlock()
	gemini.voiceCache = make(map[string][]engine.Voice)
	return nil
}

func (gemini *Gemini) Start(modelName string) error {
	return nil
}

func (gemini *Gemini) Stop(modelName string) error {
	return nil
}

func (gemini *Gemini) Play(message util.CharacterMessage) error {
	response.Debug(util.MessageData{
		Summary: "Gemini playing:" + message.Character,
		Detail:  message.Text,
	})

	input := GeminiRequest{
		Model: message.Voice.Model,
		Contents: []Content{
			{Parts: []Part{{Text: message.Text}}},
		},
		GenerationConfig: GenerationConfig{
			ResponseModalities: []string{"AUDIO"},
			SpeechConfig: SpeechConfig{
				VoiceConfig: VoiceConfig{
					PrebuiltVoiceConfig: PrebuiltVoiceConfig{
						VoiceName: message.Voice.Voice,
					},
				},
			},
		},
	}

	audioClip, err := gemini.sendRequest(input, message.Voice.Model)
	if err != nil {
		return response.Err(err)
	}

	// Assuming 24kHz 1 channel 16 bit PCM as per common Gemini/Google TTS defaults for "raw PCM"
	// Adjust if necessary based on actual API response or documentation updates.
	err = audio.PlayPCMAudioBytes(audioClip)
	if err != nil {
		return response.Err(err)
	}

	return response.Success(util.MessageData{
		Summary: "Gemini finished playing pcm",
	})
}

func (gemini *Gemini) Save(messages []util.CharacterMessage, play bool) error {
	response.Debug(util.MessageData{
		Summary: "Gemini saving messages",
	})

	err, expandedPath := util.ExpandPath(config.GetSettings().OutputPath)
	if err != nil {
		return response.Err(err)
	}

	for _, message := range messages {
		input := GeminiRequest{
			Model: message.Voice.Model,
			Contents: []Content{
				{Parts: []Part{{Text: message.Text}}},
			},
			GenerationConfig: GenerationConfig{
				ResponseModalities: []string{"AUDIO"},
				SpeechConfig: SpeechConfig{
					VoiceConfig: VoiceConfig{
						PrebuiltVoiceConfig: PrebuiltVoiceConfig{
							VoiceName: message.Voice.Voice,
						},
					},
				},
			},
		}

		audioClip, err := gemini.sendRequest(input, message.Voice.Model)
		if err != nil {
			return response.Err(err)
		}

		filename := util.GenerateFilename(
			message,
			fileIndex.Get(),
			expandedPath,
		)

		err = audio.SaveWAVFile(audioClip, filename)
		if err != nil {
			return response.Err(err)
		}

		if play {
			err = audio.PlayPCMAudioBytes(audioClip)
			if err != nil {
				return response.Err(err)
			}
		}
	}

	return nil
}

func (gemini *Gemini) Generate(model string, payload []byte) ([]byte, error) {
	var request GeminiRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, response.Err(err)
	}

	pcmData, err := gemini.sendRequest(request, model)
	if err != nil {
		return nil, response.Err(err)
	}

	return pcmData, nil
}

func (gemini *Gemini) GenerateAudio(model string, payload []byte) (*audio.Audio, error) {
	pcmData, err := gemini.Generate(model, payload)
	if err != nil {
		return nil, err
	}

	// Assuming 24kHz, 1 channel, 16 bit
	return audio.NewAudioFromPCM(pcmData, 24000, 1, 16), nil
}

func (gemini *Gemini) GetVoices(model string) ([]engine.Voice, error) {
	return []engine.Voice{
		{ID: "Zephyr", Name: "Zephyr (Bright)", Gender: ""},
		{ID: "Puck", Name: "Puck (Upbeat)", Gender: "Male"},
		{ID: "Charon", Name: "Charon (Informative)", Gender: "Male"},
		{ID: "Kore", Name: "Kore (Firm)", Gender: "Female"},
		{ID: "Fenrir", Name: "Fenrir (Excitable)", Gender: "Male"},
		{ID: "Leda", Name: "Leda (Youthful)", Gender: ""},
		{ID: "Orus", Name: "Orus (Firm)", Gender: ""},
		{ID: "Aoede", Name: "Aoede (Breezy)", Gender: "Female"},
		{ID: "Callirrhoe", Name: "Callirrhoe (Easy-going)", Gender: ""},
		{ID: "Autonoe", Name: "Autonoe (Bright)", Gender: ""},
		{ID: "Enceladus", Name: "Enceladus (Breathy)", Gender: ""},
		{ID: "Iapetus", Name: "Iapetus (Clear)", Gender: ""},
		{ID: "Umbriel", Name: "Umbriel (Easy-going)", Gender: ""},
		{ID: "Algieba", Name: "Algieba (Smooth)", Gender: ""},
		{ID: "Despina", Name: "Despina (Smooth)", Gender: ""},
		{ID: "Erinome", Name: "Erinome (Clear)", Gender: ""},
		{ID: "Algenib", Name: "Algenib (Gravelly)", Gender: ""},
		{ID: "Rasalgethi", Name: "Rasalgethi (Informative)", Gender: ""},
		{ID: "Laomedeia", Name: "Laomedeia (Upbeat)", Gender: ""},
		{ID: "Achernar", Name: "Achernar (Soft)", Gender: ""},
		{ID: "Alnilam", Name: "Alnilam (Firm)", Gender: ""},
		{ID: "Schedar", Name: "Schedar (Even)", Gender: ""},
		{ID: "Gacrux", Name: "Gacrux (Mature)", Gender: ""},
		{ID: "Pulcherrima", Name: "Pulcherrima (Forward)", Gender: ""},
		{ID: "Achird", Name: "Achird (Friendly)", Gender: ""},
		{ID: "Zubenelgenubi", Name: "Zubenelgenubi (Casual)", Gender: ""},
		{ID: "Vindemiatrix", Name: "Vindemiatrix (Gentle)", Gender: ""},
		{ID: "Sadachbia", Name: "Sadachbia (Lively)", Gender: ""},
		{ID: "Sadaltager", Name: "Sadaltager (Knowledgeable)", Gender: ""},
		{ID: "Sulafat", Name: "Sulafat (Warm)", Gender: ""},
	}, nil
}

func (gemini *Gemini) FetchModels() map[string]engine.Model {
	return FetchModels()
}

// </editor-fold>

// <editor-fold desc="Other">
func FetchModels() map[string]engine.Model {
	if config.GetEngine().Api.Gemini.ApiKey == "" {
		return make(map[string]engine.Model)
	}
	return map[string]engine.Model{
		"gemini-2.5-flash-preview-tts": {
			ID:     "gemini-2.5-flash-preview-tts",
			Name:   "Gemini 2.5 Flash Preview TTS",
			Engine: "gemini",
		},
	}
}

type Model struct {
	Voices []engine.Voice `json:"voice"`
}
