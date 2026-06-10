package openai_test

import (
	"testing"

	"nstudio/app/config"
	"nstudio/app/tts/engine/enginetest"
	"nstudio/app/tts/engine/openai"
)

func TestOpenAIEngine(t *testing.T) {
	enginetest.Configure(t)
	if config.GetEngine().Api.OpenAI.ApiKey == "" {
		t.Skip("OpenAI API key not set (OPENAI_API_KEY or app config)")
	}

	enginetest.RunInterfaceTests(t, enginetest.Case{
		Engine:   &openai.OpenAI{},
		EngineID: "openai",
		Model:    "tts-1",
	})
}
