package elevenlabs_test

import (
	"encoding/json"
	"testing"
	"time"

	"nstudio/app/config"
	"nstudio/app/tts/engine/elevenlabs"
	"nstudio/app/tts/engine/enginetest"
)

func TestElevenLabsEngine(t *testing.T) {
	outputPath := enginetest.Configure(t)
	if config.GetEngine().Api.ElevenLabs.ApiKey == "" {
		t.Skip("ElevenLabs API key not set (ELEVENLABS_API_KEY or app config)")
	}

	model := "eleven_multilingual_v2"
	enginetest.RunInterfaceTests(t, enginetest.Case{
		Engine:     &elevenlabs.ElevenLabs{},
		EngineID:   "elevenlabs",
		Model:      model,
		OutputPath: outputPath,
		Character:  "Tester",
		Pace:       2 * time.Second,
		NewPayload: func(voiceID string) []byte {
			payload, _ := json.Marshal(elevenlabs.ElevenLabsRequest{
				Text:    "Testing one two three.",
				ModelID: model,
				VoiceID: voiceID,
			})
			return payload
		},
	})
}
