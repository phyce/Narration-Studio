// Package enginetest provides shared helpers for engine integration tests.
package enginetest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"nstudio/app/common/util"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
)

const appName = "Narration Studio"

// Configure sets up an isolated, temporary config for an engine test and returns the
// temporary output directory. API keys and local-engine settings are resolved from
// environment variables first, then the user's real config file, so the tests run on a
// developer machine that already has keys configured without any extra setup. The user's
// real config is never modified (everything is written to a temp file).
func Configure(t *testing.T) string {
	t.Helper()

	info := config.Info{Name: appName, OS: runtime.GOOS}
	realConfig := readRealConfig(info)

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := config.InitializeWithPath(info, configPath); err != nil {
		t.Fatalf("initialize test config: %v", err)
	}

	outputPath := t.TempDir()

	base := config.Get()
	base.Settings.OutputPath = outputPath
	base.Engine.Local = realConfig.Engine.Local
	base.Engine.Api.OpenAI.ApiKey = resolveKey("OPENAI_API_KEY", realConfig.Engine.Api.OpenAI.ApiKey)
	base.Engine.Api.ElevenLabs.ApiKey = resolveKey("ELEVENLABS_API_KEY", realConfig.Engine.Api.ElevenLabs.ApiKey)
	base.Engine.Api.Google.ApiKey = resolveKey("GOOGLE_API_KEY", realConfig.Engine.Api.Google.ApiKey)
	base.Engine.Api.Gemini.ApiKey = resolveKey("GEMINI_API_KEY", realConfig.Engine.Api.Gemini.ApiKey)

	if err := config.Set(base); err != nil {
		t.Fatalf("apply test config: %v", err)
	}

	return outputPath
}

// Case describes one engine to exercise through the full Base interface.
type Case struct {
	Engine     engine.Base
	EngineID   string
	Model      string
	OutputPath string
	Character  string
	// NewPayload builds the Generate/GenerateAudio payload for the given voice ID.
	NewPayload func(voiceID string) []byte
}

// RunInterfaceTests exercises every engine.Base method as independent subtests, so a
// failure in one (e.g. Generate hitting an API quota) doesn't hide that the others
// (FetchModels, GetVoices, Save) work. Audio output is asserted to be non-empty.
func RunInterfaceTests(t *testing.T, c Case) {
	t.Helper()

	if err := c.Engine.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := c.Engine.Start(c.Model); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = c.Engine.Stop(c.Model) })

	t.Run("FetchModels", func(t *testing.T) {
		models := c.Engine.FetchModels()
		if len(models) == 0 {
			t.Fatal("FetchModels returned no models")
		}
		for key, model := range models {
			if model.ID == "" {
				t.Errorf("model %q has empty ID", key)
			}
			if model.Name == "" {
				t.Errorf("model %q has empty Name", key)
			}
			if c.EngineID != "" && model.Engine != c.EngineID {
				t.Errorf("model %q Engine = %q, want %q", key, model.Engine, c.EngineID)
			}
		}
	})

	var voiceID string
	t.Run("GetVoices", func(t *testing.T) {
		voices, err := c.Engine.GetVoices(c.Model)
		if err != nil {
			t.Fatalf("GetVoices: %v", err)
		}
		if len(voices) == 0 {
			t.Fatal("GetVoices returned no voices")
		}
		for index, voice := range voices {
			if voice.ID == "" {
				t.Errorf("voice %d has empty ID", index)
			}
			if voice.Name == "" {
				t.Errorf("voice %d (%s) has empty Name", index, voice.ID)
			}
		}
		voiceID = voices[0].ID
	})
	if voiceID == "" {
		return
	}

	payload := c.NewPayload(voiceID)

	t.Run("Generate", func(t *testing.T) {
		audioClip, err := c.Engine.Generate(c.Model, payload)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if len(audioClip) == 0 {
			t.Error("Generate returned empty audio")
		}
	})

	t.Run("GenerateAudio", func(t *testing.T) {
		audioObject, err := c.Engine.GenerateAudio(c.Model, payload)
		if err != nil {
			t.Fatalf("GenerateAudio: %v", err)
		}
		if audioObject == nil || len(audioObject.Data) == 0 {
			t.Error("GenerateAudio returned empty audio")
		}
	})

	message := util.CharacterMessage{
		Character: c.Character,
		Text:      "Saving audio test.",
		Voice:     util.CharacterVoice{Model: c.Model, Voice: voiceID},
	}

	t.Run("Save", func(t *testing.T) {
		if err := c.Engine.Save([]util.CharacterMessage{message}, false); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if AudioFileCount(c.OutputPath) == 0 {
			t.Error("Save did not write any audio file")
		}
	})

	if PlayEnabled() {
		t.Run("Play", func(t *testing.T) {
			if err := c.Engine.Play(message); err != nil {
				t.Errorf("Play: %v", err)
			}
		})
	}
}

// EnableModel turns on a model toggle ("engine:model") in the active test config.
func EnableModel(t *testing.T, key string) {
	t.Helper()
	base := config.Get()
	if base.ModelToggles == nil {
		base.ModelToggles = make(map[string]bool)
	}
	base.ModelToggles[key] = true
	if err := config.Set(base); err != nil {
		t.Fatalf("enable model %s: %v", key, err)
	}
}

// AudioFileCount returns the number of non-empty files anywhere under dir.
func AudioFileCount(dir string) int {
	count := 0
	_ = filepath.Walk(dir, func(path string, fileInfo os.FileInfo, err error) error {
		if err == nil && !fileInfo.IsDir() && fileInfo.Size() > 0 {
			count++
		}
		return nil
	})
	return count
}

// PlayEnabled reports whether playback-exercising tests should run. Playback emits real
// audio, so it is opt-in via NSTUDIO_TEST_PLAY to keep normal test runs silent.
func PlayEnabled() bool {
	return os.Getenv("NSTUDIO_TEST_PLAY") != ""
}

func resolveKey(environmentVariable, fallback string) string {
	if value := os.Getenv(environmentVariable); value != "" {
		return value
	}
	return fallback
}

func readRealConfig(info config.Info) config.Base {
	var base config.Base
	configDir, err := os.UserConfigDir()
	if err != nil {
		return base
	}
	data, err := os.ReadFile(filepath.Join(configDir, info.Name, "config.json"))
	if err != nil {
		return base
	}
	_ = json.Unmarshal(data, &base)
	return base
}
