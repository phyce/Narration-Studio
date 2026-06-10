package piper_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"nstudio/app/common/util"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/engine/enginetest"
	"nstudio/app/tts/engine/piper"
	"nstudio/app/tts/engine/piper/native"
)

func TestPiperEngine(t *testing.T) {
	enginetest.Configure(t)

	settings := config.GetEngine().Local.Piper

	err, modelsPath := util.ExpandPath(settings.ModelsDirectory)
	if err != nil || modelsPath == "" {
		t.Skip("Piper models directory not configured")
	}

	model := findInstalledModel(modelsPath)
	if model == "" {
		t.Skipf("no installed Piper model found in %s", modelsPath)
	}

	executablePath := expandExecutable(t, settings.Location)
	useExecutable := settings.UseExecutable
	if !useExecutable && !native.IsNativeAvailable() {
		if !fileExists(executablePath) {
			t.Skip("Piper native runtime unavailable and executable not found")
		}
		useExecutable = true
	}
	if useExecutable && !fileExists(executablePath) {
		t.Skipf("Piper executable not found: %s", executablePath)
	}
	if useExecutable != settings.UseExecutable {
		base := config.Get()
		base.Engine.Local.Piper.UseExecutable = useExecutable
		if err := config.Set(base); err != nil {
			t.Fatalf("set piper executable mode: %v", err)
		}
	}

	enginetest.EnableModel(t, "piper:"+model)

	piperEngine := &piper.Piper{}
	if err := piperEngine.Initialize(); err != nil {
		t.Skipf("Piper Initialize failed (runtime not set up): %v", err)
	}
	if err := piperEngine.Start(model); err != nil {
		t.Skipf("Piper Start failed (runtime not set up): %v", err)
	}
	t.Cleanup(func() { _ = piperEngine.Stop(model) })

	t.Run("FetchModels", func(t *testing.T) {
		models := piperEngine.FetchModels()
		if len(models) == 0 {
			t.Fatal("FetchModels returned no models")
		}
		for key, m := range models {
			if m.ID == "" {
				t.Errorf("model %q has empty ID", key)
			}
			if m.Name == "" {
				t.Errorf("model %q has empty Name", key)
			}
			if m.Engine != "piper" {
				t.Errorf("model %q Engine = %q, want %q", key, m.Engine, "piper")
			}
		}
	})

	var voiceID string
	t.Run("GetVoices", func(t *testing.T) {
		voices, err := piperEngine.GetVoices(model)
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
		stream, err := piperEngine.Synthesize(context.Background(), engine.SpeechRequest{
			Model: model,
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

func TestPiperBinarySynthesizeStreams(t *testing.T) {
	enginetest.Configure(t)

	settings := config.GetEngine().Local.Piper

	err, modelsPath := util.ExpandPath(settings.ModelsDirectory)
	if err != nil || modelsPath == "" {
		t.Skip("Piper models directory not configured")
	}

	model := findInstalledModel(modelsPath)
	if model == "" {
		t.Skipf("no installed Piper model found in %s", modelsPath)
	}

	executablePath := expandExecutable(t, settings.Location)
	if !fileExists(executablePath) {
		t.Skipf("Piper executable not found: %s", executablePath)
	}

	base := config.Get()
	base.Engine.Local.Piper.UseExecutable = true
	if err := config.Set(base); err != nil {
		t.Fatalf("set piper executable mode: %v", err)
	}

	enginetest.EnableModel(t, "piper:"+model)

	piperEngine := &piper.Piper{}
	if err := piperEngine.Initialize(); err != nil {
		t.Skipf("Piper Initialize failed (runtime not set up): %v", err)
	}
	if err := piperEngine.Start(model); err != nil {
		t.Skipf("Piper Start failed (runtime not set up): %v", err)
	}
	t.Cleanup(func() { _ = piperEngine.Stop(model) })

	voices, err := piperEngine.GetVoices(model)
	if err != nil {
		t.Fatalf("GetVoices: %v", err)
	}
	if len(voices) == 0 {
		t.Fatal("GetVoices returned no voices")
	}

	for index, text := range []string{"Testing one two three.", "Second utterance on the same process."} {
		stream, err := piperEngine.Synthesize(context.Background(), engine.SpeechRequest{
			Model: model,
			Voice: voices[0].ID,
			Text:  text,
		})
		if err != nil {
			t.Fatalf("Synthesize %d: %v", index, err)
		}

		audioObject, err := stream.ReadAll()
		if err != nil {
			t.Fatalf("ReadAll %d: %v", index, err)
		}
		if len(audioObject.Data) == 0 {
			t.Errorf("Synthesize %d returned empty audio", index)
		}
		if audioObject.Metadata.SampleRate <= 0 {
			t.Errorf("Synthesize %d returned invalid sample rate %d", index, audioObject.Metadata.SampleRate)
		}

		select {
		case <-stream.Done():
		default:
			t.Errorf("stream %d Done() not closed after full drain", index)
		}
	}
}

func findInstalledModel(modelsPath string) string {
	entries, err := os.ReadDir(modelsPath)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		base := filepath.Join(modelsPath, name, name)
		if fileExists(base+".onnx") && fileExists(base+".onnx.json") && fileExists(base+".metadata.json") {
			return name
		}
	}
	return ""
}

func expandExecutable(t *testing.T, path string) string {
	t.Helper()
	err, expanded := util.ExpandPath(path)
	if err != nil {
		return path
	}
	return expanded
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
