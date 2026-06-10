package google_test

import (
	"testing"

	"nstudio/app/config"
	"nstudio/app/tts/engine/enginetest"
	"nstudio/app/tts/engine/google"
)

func TestGoogleEngine(t *testing.T) {
	enginetest.Configure(t)
	if config.GetEngine().Api.Google.ApiKey == "" {
		t.Skip("Google API key not set (GOOGLE_API_KEY or app config)")
	}

	enginetest.RunInterfaceTests(t, enginetest.Case{
		Engine:   &google.Google{},
		EngineID: "google",
		Model:    "standard",
	})
}
