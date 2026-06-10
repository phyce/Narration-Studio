package elevenlabs_test

import (
	"testing"

	"nstudio/app/config"
	"nstudio/app/tts/engine/elevenlabs"
	"nstudio/app/tts/engine/enginetest"
)

func TestElevenLabsEngine(t *testing.T) {
	enginetest.Configure(t)
	if config.GetEngine().Api.ElevenLabs.ApiKey == "" {
		t.Skip("ElevenLabs API key not set (ELEVENLABS_API_KEY or app config)")
	}

	enginetest.RunInterfaceTests(t, enginetest.Case{
		Engine:   &elevenlabs.ElevenLabs{},
		EngineID: "elevenlabs",
		Model:    "eleven_multilingual_v2",
	})
}
