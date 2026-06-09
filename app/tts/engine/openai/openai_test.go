package openai_test

import (
	"encoding/json"
	"testing"
	"time"

	"nstudio/app/config"
	"nstudio/app/tts/engine/enginetest"
	"nstudio/app/tts/engine/openai"
)

func TestOpenAIEngine(t *testing.T) {
	outputPath := enginetest.Configure(t)
	if config.GetEngine().Api.OpenAI.ApiKey == "" {
		t.Skip("OpenAI API key not set (OPENAI_API_KEY or app config)")
	}

	enginetest.RunInterfaceTests(t, enginetest.Case{
		Engine:     &openai.OpenAI{},
		EngineID:   "openai",
		Model:      "tts-1",
		OutputPath: outputPath,
		Character:  "Tester",
		Pace:       2 * time.Second,
		NewPayload: func(voiceID string) []byte {
			payload, _ := json.Marshal(openai.OpenAIRequest{
				Model: "tts-1",
				Input: "Testing one two three.",
				Voice: voiceID,
			})
			return payload
		},
	})
}
