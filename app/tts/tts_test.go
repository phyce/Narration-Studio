package tts_test

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"nstudio/app/common/util"
	"nstudio/app/common/util/fileIndex"
	"nstudio/app/tts"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/engine/enginetest"
	"nstudio/app/tts/engine/mssapi5"
	"nstudio/app/tts/modelManager"
	"nstudio/app/tts/profile"
)

func TestMain(m *testing.M) {
	profilesPath, err := os.MkdirTemp("", "nstudio-test-profiles")
	if err != nil {
		panic(err)
	}

	if err := profile.InitializeProfileDirectory(profilesPath); err != nil {
		panic(err)
	}

	exitCode := enginetest.RunWithTemporaryConfig(m)
	os.RemoveAll(profilesPath)
	os.Exit(exitCode)
}

func TestGenerateSpeechPipeline(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("pipeline test uses the SAPI5 engine (Windows only)")
	}

	outputPath := configureSAPI5(t)

	messages := []util.CharacterMessage{
		{Character: "Alice", Text: "First line of the script."},
		{Character: "Bob", Text: "Second line follows immediately."},
		{Character: "Alice", Text: "Third line closes the scene."},
	}

	if err := tts.GenerateSpeech(messages, true, "pipeline-test"); err != nil {
		t.Fatalf("GenerateSpeech: %v", err)
	}

	if count := enginetest.AudioFileCount(outputPath); count != 3 {
		t.Errorf("expected 3 saved audio files, found %d", count)
	}
}

func configureSAPI5(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("pipeline test uses the SAPI5 engine (Windows only)")
	}

	outputPath := enginetest.UseNewOutputPath(t)

	enginetest.EnableModel(t, "mssapi5:mssapi5")
	modelManager.Initialize(true)
	if err := modelManager.RegisterEngine(engine.Engine{
		ID:     "mssapi5",
		Name:   "Microsoft SAPI5",
		Type:   engine.Local,
		Tags:   []string{"local"},
		Engine: &mssapi5.MsSapi5{},
		Models: mssapi5.FetchModels(),
	}); err != nil {
		t.Fatalf("RegisterEngine: %v", err)
	}

	fileIndex.Reset()
	return outputPath
}

func numberedLines(count int) []util.CharacterMessage {
	messages := make([]util.CharacterMessage, 0, count)
	for index := range count {
		messages = append(messages, util.CharacterMessage{
			Character: "Narrator",
			Text:      fmt.Sprintf("This is line number %d of a longer script.", index+1),
		})
	}
	return messages
}

func generateInBackground(messages []util.CharacterMessage, profileID string) chan error {
	result := make(chan error, 1)
	go func() {
		result <- tts.GenerateSpeech(messages, true, profileID)
	}()
	return result
}

func waitForFileCount(t *testing.T, outputPath string, minimum int) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for enginetest.AudioFileCount(outputPath) < minimum {
		if time.Now().After(deadline) {
			t.Fatalf("fewer than %d files saved after 30s", minimum)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestGenerateSpeechReturnsErrStoppedWhenStopped(t *testing.T) {
	outputPath := configureSAPI5(t)
	const lineCount = 40

	result := generateInBackground(numberedLines(lineCount), "stop-test")
	waitForFileCount(t, outputPath, 1)
	tts.Stop()

	select {
	case err := <-result:
		if !errors.Is(err, tts.ErrStopped) {
			t.Fatalf("GenerateSpeech returned %v, want ErrStopped", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("GenerateSpeech did not return after Stop")
	}

	if count := enginetest.AudioFileCount(outputPath); count >= lineCount {
		t.Errorf("all %d lines were saved despite Stop", count)
	}

	waitForEngineIdle(t, "mssapi5", "mssapi5")
}

func waitForEngineIdle(t *testing.T, engineID, modelID string) {
	t.Helper()
	_, release, ok := modelManager.GetEngineInstance(engineID, modelID)
	if !ok {
		t.Fatalf("no %s:%s instance to wait on", engineID, modelID)
	}
	release()
}

func TestPauseHoldsSaveRunUntilResumed(t *testing.T) {
	outputPath := configureSAPI5(t)
	const lineCount = 12

	result := generateInBackground(numberedLines(lineCount), "pause-test")
	waitForFileCount(t, outputPath, 1)
	tts.SetPaused(true)

	time.Sleep(1500 * time.Millisecond)
	countWhilePaused := enginetest.AudioFileCount(outputPath)
	time.Sleep(time.Second)
	if count := enginetest.AudioFileCount(outputPath); count != countWhilePaused {
		t.Fatalf("files kept being saved while paused: %d, then %d", countWhilePaused, count)
	}
	if countWhilePaused >= lineCount {
		t.Fatalf("all %d lines were saved before the pause took effect", countWhilePaused)
	}

	tts.SetPaused(false)

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("GenerateSpeech after resume: %v", err)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("GenerateSpeech did not finish after resume")
	}
	if count := enginetest.AudioFileCount(outputPath); count != lineCount {
		t.Errorf("saved %d files, want %d", count, lineCount)
	}
}
