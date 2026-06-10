package gemini_test

import (
	"testing"

	"nstudio/app/config"
	"nstudio/app/tts/engine/enginetest"
	"nstudio/app/tts/engine/gemini"
)

func TestGeminiEngine(t *testing.T) {
	enginetest.Configure(t)
	if config.GetEngine().Api.Gemini.ApiKey == "" {
		t.Skip("Gemini API key not set (GEMINI_API_KEY or app config)")
	}

	enginetest.RunInterfaceTests(t, enginetest.Case{
		Engine:   &gemini.Gemini{},
		EngineID: "gemini",
		Model:    "gemini-2.5-flash-preview-tts",
	})
}
