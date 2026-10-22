package enginetest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"nstudio/app/config"
	"nstudio/app/tts/engine"

	"github.com/charmbracelet/log"
)

const appName = "Narration Studio"

func Configure(t *testing.T) string {
	t.Helper()

	if err := initializeTemporaryConfig(filepath.Join(t.TempDir(), "config.json")); err != nil {
		t.Fatalf("initialize test config: %v", err)
	}

	return UseNewOutputPath(t)
}

func RunWithTemporaryConfig(m *testing.M) int {
	configRootPath, err := os.MkdirTemp("", "nstudio-test-config")
	if err != nil {
		fmt.Fprintln(os.Stderr, "create test config folder:", err)
		return 1
	}
	defer os.RemoveAll(configRootPath)

	if err := initializeTemporaryConfig(filepath.Join(configRootPath, "config.json")); err != nil {
		fmt.Fprintln(os.Stderr, "initialize test config:", err)
		return 1
	}

	return m.Run()
}

func UseNewOutputPath(t *testing.T) string {
	t.Helper()
	outputPath := t.TempDir()

	base := config.Get()
	base.Settings.OutputPath = outputPath
	if err := config.Set(base); err != nil {
		t.Fatalf("set test output path: %v", err)
	}

	return outputPath
}

func initializeTemporaryConfig(configPath string) error {
	log.SetLevel(log.FatalLevel)

	info := config.Info{Name: appName, OS: runtime.GOOS}
	realConfig := readRealConfig(info)

	if err := config.InitializeWithPath(info, configPath); err != nil {
		return err
	}

	base := config.Get()
	base.Engine.Local = realConfig.Engine.Local
	base.Engine.Api.OpenAI.ApiKey = resolveKey("OPENAI_API_KEY", realConfig.Engine.Api.OpenAI.ApiKey)
	base.Engine.Api.ElevenLabs.ApiKey = resolveKey("ELEVENLABS_API_KEY", realConfig.Engine.Api.ElevenLabs.ApiKey)
	base.Engine.Api.Google.ApiKey = resolveKey("GOOGLE_API_KEY", realConfig.Engine.Api.Google.ApiKey)
	base.Engine.Api.Gemini.ApiKey = resolveKey("GEMINI_API_KEY", realConfig.Engine.Api.Gemini.ApiKey)

	return config.Set(base)
}

type Case struct {
	Engine   engine.Base
	EngineID string
	Model    string
}

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

	t.Run("Synthesize", func(t *testing.T) {
		stream, err := c.Engine.Synthesize(context.Background(), engine.SpeechRequest{
			Model: c.Model,
			Voice: voiceID,
			Text:  "Testing one two three.",
		})
		if err != nil {
			t.Fatalf("Synthesize: %v", err)
		}
		audioObject, err := stream.ReadAll()
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if audioObject == nil || len(audioObject.Data) == 0 {
			t.Error("Synthesize returned empty audio")
		}
		if audioObject != nil && audioObject.Metadata.SampleRate <= 0 {
			t.Errorf("Synthesize returned invalid sample rate %d", audioObject.Metadata.SampleRate)
		}
	})
}

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
