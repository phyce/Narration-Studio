package google_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"nstudio/app/config"
	"nstudio/app/tts/engine/enginetest"
	"nstudio/app/tts/engine/google"
)

func TestGoogleEngine(t *testing.T) {
	outputPath := enginetest.Configure(t)
	if config.GetEngine().Api.Google.ApiKey == "" {
		t.Skip("Google API key not set (GOOGLE_API_KEY or app config)")
	}

	enginetest.RunInterfaceTests(t, enginetest.Case{
		Engine:     &google.Google{},
		EngineID:   "google",
		Model:      "standard",
		OutputPath: outputPath,
		Character:  "Tester",
		Pace:       2 * time.Second,
		NewPayload: func(voiceID string) []byte {
			languageCode := "en-US"
			if parts := strings.Split(voiceID, "-"); len(parts) >= 2 {
				languageCode = parts[0] + "-" + parts[1]
			}
			payload, _ := json.Marshal(google.GoogleRequest{
				Input: google.Input{Text: "Testing one two three."},
				Voice: google.VoiceSelectionParams{Name: voiceID, LanguageCode: languageCode},
				AudioConfig: google.AudioConfig{
					AudioEncoding: "LINEAR16",
					SpeakingRate:  1,
					Pitch:         0,
				},
			})
			return payload
		},
	})
}
