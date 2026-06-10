package gemini

import (
	"context"
	"nstudio/app/common/audio"
	"nstudio/app/common/response"
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

func (gemini *Gemini) Synthesize(ctx context.Context, request engine.SpeechRequest) (*audio.Stream, error) {
	input := GeminiRequest{
		Model: request.Model,
		Contents: []Content{
			{Parts: []Part{{Text: request.Text}}},
		},
		GenerationConfig: GenerationConfig{
			ResponseModalities: []string{"AUDIO"},
			SpeechConfig: SpeechConfig{
				VoiceConfig: VoiceConfig{
					PrebuiltVoiceConfig: PrebuiltVoiceConfig{
						VoiceName: request.Voice,
					},
				},
			},
		},
	}

	pcmData, err := gemini.sendRequest(ctx, input, request.Model)
	if err != nil {
		return nil, response.Err("%w", err)
	}

	return audio.NewBufferedStream(audio.StreamFormat{SampleRate: 24000, Channels: 1, BitDepth: 16}, pcmData), nil
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
