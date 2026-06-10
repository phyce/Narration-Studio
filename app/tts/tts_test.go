package tts_test

import (
	"runtime"
	"testing"

	"nstudio/app/common/util"
	"nstudio/app/common/util/fileIndex"
	"nstudio/app/tts"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/engine/enginetest"
	"nstudio/app/tts/engine/mssapi5"
	"nstudio/app/tts/modelManager"
)

func TestGenerateSpeechPipeline(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("pipeline test uses the SAPI5 engine (Windows only)")
	}

	outputPath := enginetest.Configure(t)

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
