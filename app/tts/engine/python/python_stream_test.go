package python

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
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
	os.Exit(enginetest.RunWithTemporaryConfig(m))
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
			if os.Getenv("NSTUDIO_FAKE_LOAD") == "fail" {
				emit(map[string]interface{}{"id": request.ID, "event": "error", "error": "model file missing"})
				continue
			}
			emit(map[string]interface{}{"id": request.ID, "event": "done", "sampleRate": 24000, "channels": 1, "bits": 16})
		case "synth":
			if request.Voice == "explode" {
				emit(map[string]interface{}{"id": request.ID, "event": "error", "error": "synthetic failure"})
				continue
			}
			if os.Getenv("NSTUDIO_FAKE_SYNTH_VOICE_LOG") != "" {
				os.WriteFile(os.Getenv("NSTUDIO_FAKE_SYNTH_VOICE_LOG"), []byte(request.Voice), 0o644)
			}
			pcm := fakePCMForText(request.Text)
			if os.Getenv("NSTUDIO_FAKE_SYNTH") == "silent" {
				pcm = make([]byte, len(pcm))
			}
			half := len(pcm) / 2
			os.Stdout.Write(pcm[:half])
			time.Sleep(20 * time.Millisecond)
			os.Stdout.Write(pcm[half:])
			emit(map[string]interface{}{
				"id": request.ID, "event": "done",
				"sampleRate": 24000, "channels": 1, "bits": 16,
				"totalSamples": len(pcm) / 2,
			})
		case "prepare":
			switch os.Getenv("NSTUDIO_FAKE_PREPARE") {
			case "fail":
				emit(map[string]interface{}{"id": request.ID, "event": "error", "error": "network unreachable"})
			case "stall":
			default:
				for downloaded := 0; downloaded <= 300; downloaded += 100 {
					emit(map[string]interface{}{"id": request.ID, "event": "progress", "downloaded": downloaded, "total": 300})
					time.Sleep(10 * time.Millisecond)
				}
				emit(map[string]interface{}{"id": request.ID, "event": "done"})
			}
		case "shutdown":
			emit(map[string]interface{}{"id": request.ID, "event": "done"})
			return
		}
	}
}

func fakeRunnerManifest(t *testing.T, extraEnvironment map[string]string) (Manifest, string) {
	t.Helper()
	executablePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	environment := map[string]string{"NSTUDIO_FAKE_STREAM_RUNNER": "1"}
	for name, value := range extraEnvironment {
		environment[name] = value
	}

	return Manifest{
		ID:   "fake-pack",
		Name: "Fake Pack",
		Runtime: Runtime{
			Python:     filepath.Base(executablePath),
			Entrypoint: filepath.Base(executablePath),
			Env:        environment,
		},
	}, filepath.Dir(executablePath)
}

func TestPrepareBundle(t *testing.T) {
	t.Run("ReportsProgressUntilDone", func(t *testing.T) {
		manifest, bundlePath := fakeRunnerManifest(t, nil)

		var reported [][2]int64
		err := PrepareBundle(context.Background(), manifest, bundlePath, func(downloaded, total int64) {
			reported = append(reported, [2]int64{downloaded, total})
		})
		if err != nil {
			t.Fatalf("PrepareBundle: %v", err)
		}
		if len(reported) != 4 || reported[len(reported)-1] != [2]int64{300, 300} {
			t.Errorf("progress = %v, want four reports ending at 300 of 300", reported)
		}
	})

	t.Run("SurfacesRunnerError", func(t *testing.T) {
		manifest, bundlePath := fakeRunnerManifest(t, map[string]string{"NSTUDIO_FAKE_PREPARE": "fail"})

		err := PrepareBundle(context.Background(), manifest, bundlePath, func(int64, int64) {})
		if err == nil || !strings.Contains(err.Error(), "network unreachable") {
			t.Errorf("PrepareBundle error = %v, want the runner's error", err)
		}
	})

	t.Run("FailsWhenDownloadStalls", func(t *testing.T) {
		previousStallTimeout := prepareStallTimeout
		prepareStallTimeout = 300 * time.Millisecond
		t.Cleanup(func() { prepareStallTimeout = previousStallTimeout })

		manifest, bundlePath := fakeRunnerManifest(t, map[string]string{"NSTUDIO_FAKE_PREPARE": "stall"})

		err := PrepareBundle(context.Background(), manifest, bundlePath, func(int64, int64) {})
		if !errors.Is(err, ErrRunnerTimeout) {
			t.Errorf("PrepareBundle error = %v, want ErrRunnerTimeout", err)
		}
	})

	t.Run("StopsWhenCancelled", func(t *testing.T) {
		manifest, bundlePath := fakeRunnerManifest(t, map[string]string{"NSTUDIO_FAKE_PREPARE": "stall"})
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(300*time.Millisecond, cancel)

		started := time.Now()
		err := PrepareBundle(ctx, manifest, bundlePath, func(int64, int64) {})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("PrepareBundle error = %v, want context.Canceled", err)
		}
		if elapsed := time.Since(started); elapsed > 10*time.Second {
			t.Errorf("PrepareBundle took %s to stop after cancel", elapsed)
		}
	})
}

func shortenVerificationDuration(t *testing.T) {
	previousDuration := minimumVerificationDuration
	minimumVerificationDuration = 10 * time.Millisecond
	t.Cleanup(func() { minimumVerificationDuration = previousDuration })
}

func TestVerifyBundle(t *testing.T) {
	shortenVerificationDuration(t)

	t.Run("PassesWhenTheModelSpeaks", func(t *testing.T) {
		manifest, bundlePath := fakeRunnerManifest(t, nil)
		manifest.Models = []ManifestModel{{ID: "fake-model", Voices: []ManifestVoice{{ID: "alice"}}}}

		if err := VerifyBundle(context.Background(), manifest, bundlePath); err != nil {
			t.Fatalf("VerifyBundle: %v", err)
		}
	})

	t.Run("UsesTheBundledTestVoiceForCloningModels", func(t *testing.T) {
		voiceLogPath := filepath.Join(t.TempDir(), "voice.txt")
		manifest, bundlePath := fakeRunnerManifest(t, map[string]string{"NSTUDIO_FAKE_SYNTH_VOICE_LOG": voiceLogPath})
		manifest.Models = []ManifestModel{{ID: "cloning-model", TestVoice: "selftest.wav"}}

		if err := VerifyBundle(context.Background(), manifest, bundlePath); err != nil {
			t.Fatalf("VerifyBundle: %v", err)
		}
		usedVoice, _ := os.ReadFile(voiceLogPath)
		if want := filepath.Join(bundlePath, "selftest.wav"); string(usedVoice) != want {
			t.Errorf("synthesized with voice %q, want %q", usedVoice, want)
		}
	})

	t.Run("FailsOnSilence", func(t *testing.T) {
		manifest, bundlePath := fakeRunnerManifest(t, map[string]string{"NSTUDIO_FAKE_SYNTH": "silent"})
		manifest.Models = []ManifestModel{{ID: "fake-model", Voices: []ManifestVoice{{ID: "alice"}}}}

		err := VerifyBundle(context.Background(), manifest, bundlePath)
		if err == nil || !strings.Contains(err.Error(), "no audible audio") {
			t.Errorf("VerifyBundle error = %v, want a no-audio failure", err)
		}
	})

	t.Run("FailsWhenTheModelDoesNotLoad", func(t *testing.T) {
		manifest, bundlePath := fakeRunnerManifest(t, map[string]string{"NSTUDIO_FAKE_LOAD": "fail"})
		manifest.Models = []ManifestModel{{ID: "fake-model", Voices: []ManifestVoice{{ID: "alice"}}}}

		err := VerifyBundle(context.Background(), manifest, bundlePath)
		if err == nil || !strings.Contains(err.Error(), "model file missing") {
			t.Errorf("VerifyBundle error = %v, want the load failure", err)
		}
	})

	t.Run("FailsWhenSynthesisFails", func(t *testing.T) {
		manifest, bundlePath := fakeRunnerManifest(t, nil)
		manifest.Models = []ManifestModel{{ID: "fake-model", Voices: []ManifestVoice{{ID: "explode"}}}}

		err := VerifyBundle(context.Background(), manifest, bundlePath)
		if err == nil || !strings.Contains(err.Error(), "synthetic failure") {
			t.Errorf("VerifyBundle error = %v, want the synthesis failure", err)
		}
	})
}

func TestPythonStreamSynthesize(t *testing.T) {
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

func TestExecutableRunnerSpeaksEachVoiceInItsOwnFormat(t *testing.T) {
	executablePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	manifest := Manifest{
		ID:   "fake-native-pack",
		Name: "Fake Native Pack",
		Runtime: Runtime{
			Executable: filepath.Base(executablePath),
			Env:        map[string]string{"NSTUDIO_FAKE_STREAM_RUNNER": "1"},
		},
		Models: []ManifestModel{{
			ID:   "fake-model",
			Name: "Fake Model",
			Voices: []ManifestVoice{
				{ID: "wide", Name: "Wide Voice"},
				{ID: "narrow", Name: "Narrow Voice", SampleRate: 8000},
			},
		}},
	}
	nativeEngine := New(manifest, filepath.Dir(executablePath))
	if err := nativeEngine.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	t.Cleanup(func() { _ = nativeEngine.Stop("") })

	for _, testCase := range []struct {
		voice              string
		expectedSampleRate int
	}{
		{voice: "wide", expectedSampleRate: 24000},
		{voice: "narrow", expectedSampleRate: 8000},
		{voice: "Narrow Voice", expectedSampleRate: 8000},
	} {
		t.Run(testCase.voice, func(t *testing.T) {
			stream, err := nativeEngine.Synthesize(context.Background(), engine.SpeechRequest{Model: "fake-model", Voice: testCase.voice, Text: "hello"})
			if err != nil {
				t.Fatalf("Synthesize: %v", err)
			}
			audioObject, err := stream.ReadAll()
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}
			if audioObject.Metadata.SampleRate != testCase.expectedSampleRate {
				t.Errorf("sample rate = %d, want %d", audioObject.Metadata.SampleRate, testCase.expectedSampleRate)
			}
			if expected := fakePCMForText("hello"); !bytes.Equal(audioObject.Data, expected) {
				t.Errorf("PCM mismatch: got %d bytes, want %d", len(audioObject.Data), len(expected))
			}
		})
	}
}

func TestExecutableRunnerGetsNoPythonEnvironment(t *testing.T) {
	t.Setenv("PYTHONPATH", "parent-python-path")
	environment := childProcessEnvironment("bundle", Runtime{Executable: "runner.exe", Env: map[string]string{"RUNNER_SETTING": "on"}})

	for _, expectedEntry := range []string{"PYTHONPATH=parent-python-path", "RUNNER_SETTING=on"} {
		if !slices.Contains(environment, expectedEntry) {
			t.Errorf("environment lacks %s", expectedEntry)
		}
	}
	if slices.Contains(environment, "PYTHONUNBUFFERED=1") {
		t.Error("an executable runner got the Python runner environment")
	}
}

type engineStreamResult struct {
	data       []byte
	sampleRate int
	channels   int
	bitDepth   int
}
