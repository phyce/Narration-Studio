package gemini_test

import (
	"encoding/json"
	"testing"
	"time"

	"nstudio/app/config"
	"nstudio/app/tts/engine/enginetest"
	"nstudio/app/tts/engine/gemini"
)

func TestGeminiEngine(t *testing.T) {
	outputPath := enginetest.Configure(t)
	if config.GetEngine().Api.Gemini.ApiKey == "" {
		t.Skip("Gemini API key not set (GEMINI_API_KEY or app config)")
	}

	model := "gemini-2.5-flash-preview-tts"
	enginetest.RunInterfaceTests(t, enginetest.Case{
		Engine:     &gemini.Gemini{},
		EngineID:   "gemini",
		Model:      model,
		OutputPath: outputPath,
		Character:  "Tester",
		Pace:       30 * time.Second,
		NewPayload: func(voiceID string) []byte {
			payload, _ := json.Marshal(gemini.GeminiRequest{
				Model:    model,
				Contents: []gemini.Content{{Parts: []gemini.Part{{Text: "Testing one two three."}}}},
				GenerationConfig: gemini.GenerationConfig{
					ResponseModalities: []string{"AUDIO"},
					SpeechConfig: gemini.SpeechConfig{
						VoiceConfig: gemini.VoiceConfig{
							PrebuiltVoiceConfig: gemini.PrebuiltVoiceConfig{VoiceName: voiceID},
						},
					},
				},
			})
			return payload
		},
	})
}
