package python

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nstudio/app/tts/engine"
	"nstudio/app/tts/engine/enginetest"
)

// TestMain doubles as the pack runner when re-executed by the test itself:
// the manifest's Runtime.Env injects NSTUDIO_FAKE_STREAM_RUNNER, so the
// spawned "python interpreter" is this binary speaking the streaming protocol.
func TestMain(m *testing.M) {
	if os.Getenv("NSTUDIO_FAKE_STREAM_RUNNER") == "1" {
		runFakeStreamRunner()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakePCMForText(text string) []byte {
	pcm := make([]byte, 64*len(text))
	for index := range pcm {
		pcm[index] = byte((index + len(text)) % 251)
	}
	return pcm
}

func runFakeStreamRunner() {
	emit := func(event map[string]interface{}) {
		line, _ := json.Marshal(event)
		os.Stderr.Write(append(line, '\n'))
	}

	os.Stderr.WriteString("[fake-runner] booting\n")
	emit(map[string]interface{}{"event": "ready", "runner": "fake", "version": "0.0.1"})

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var request struct {
			ID    string `json:"id"`
			Op    string `json:"op"`
			Voice string `json:"voice"`
			Text  string `json:"text"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			continue
		}
		switch request.Op {
		case "load":
			emit(map[string]interface{}{"id": request.ID, "event": "done", "sampleRate": 24000, "channels": 1, "bits": 16})
		case "synth":
			if request.Voice == "explode" {
				emit(map[string]interface{}{"id": request.ID, "event": "error", "error": "synthetic failure"})
				continue
			}
			pcm := fakePCMForText(request.Text)
			half := len(pcm) / 2
			os.Stdout.Write(pcm[:half])
			time.Sleep(20 * time.Millisecond)
			os.Stdout.Write(pcm[half:])
			emit(map[string]interface{}{
				"id": request.ID, "event": "done",
				"sampleRate": 24000, "channels": 1, "bits": 16,
				"totalSamples": len(pcm) / 2,
			})
		case "shutdown":
			emit(map[string]interface{}{"id": request.ID, "event": "done"})
			return
		}
	}
}

func TestPythonStreamSynthesize(t *testing.T) {
	enginetest.Configure(t)

	executablePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	manifest := Manifest{
		ID:   "fake-pack",
		Name: "Fake Pack",
		Runtime: Runtime{
			Python:     filepath.Base(executablePath),
			Entrypoint: filepath.Base(executablePath),
			Env:        map[string]string{"NSTUDIO_FAKE_STREAM_RUNNER": "1"},
		},
		Models: []ManifestModel{{
			ID:     "fake-model",
			Name:   "Fake Model",
			Voices: []ManifestVoice{{ID: "af_test", Name: "Test Voice"}},
		}},
	}
	pythonEngine := New(manifest, filepath.Dir(executablePath))
	t.Cleanup(func() { _ = pythonEngine.Stop("") })

	synthesize := func(t *testing.T, voice, text string) (*engineStreamResult, error) {
		t.Helper()
		stream, err := pythonEngine.Synthesize(context.Background(), engine.SpeechRequest{
			Model: "fake-model",
			Voice: voice,
			Text:  text,
		})
		if err != nil {
			return nil, err
		}
		audioObject, err := stream.ReadAll()
		if err != nil {
			return nil, err
		}
		return &engineStreamResult{
			data:       audioObject.Data,
			sampleRate: audioObject.Metadata.SampleRate,
			channels:   audioObject.Metadata.Channels,
			bitDepth:   audioObject.Metadata.BitDepth,
		}, nil
	}

	t.Run("StreamsUtterance", func(t *testing.T) {
		result, err := synthesize(t, "af_test", "hello world")
		if err != nil {
			t.Fatalf("Synthesize: %v", err)
		}
		if result.sampleRate != 24000 || result.channels != 1 || result.bitDepth != 16 {
			t.Errorf("format = %d/%d/%d, want 24000/1/16", result.sampleRate, result.channels, result.bitDepth)
		}
		if expected := fakePCMForText("hello world"); !bytes.Equal(result.data, expected) {
			t.Errorf("PCM mismatch: got %d bytes, want %d", len(result.data), len(expected))
		}
	})

	t.Run("SecondUtteranceIsClean", func(t *testing.T) {
		result, err := synthesize(t, "af_test", "a different, longer second utterance")
		if err != nil {
			t.Fatalf("Synthesize: %v", err)
		}
		if expected := fakePCMForText("a different, longer second utterance"); !bytes.Equal(result.data, expected) {
			t.Errorf("PCM mismatch: got %d bytes, want %d", len(result.data), len(expected))
		}
	})

	t.Run("ErrorEventFailsStream", func(t *testing.T) {
		_, err := synthesize(t, "explode", "this should fail")
		if err == nil {
			t.Fatal("expected error from error event, got nil")
		}
		if !strings.Contains(err.Error(), "synthetic failure") {
			t.Errorf("error = %q, want it to contain %q", err, "synthetic failure")
		}
	})

	t.Run("RecoversAfterError", func(t *testing.T) {
		result, err := synthesize(t, "af_test", "back to normal")
		if err != nil {
			t.Fatalf("Synthesize: %v", err)
		}
		if expected := fakePCMForText("back to normal"); !bytes.Equal(result.data, expected) {
			t.Errorf("PCM mismatch: got %d bytes, want %d", len(result.data), len(expected))
		}
	})
}

type engineStreamResult struct {
	data       []byte
	sampleRate int
	channels   int
	bitDepth   int
}
