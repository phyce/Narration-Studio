package mssapi5_test

import (
	"encoding/json"
	"runtime"
	"testing"

	"nstudio/app/tts/engine/enginetest"
	"nstudio/app/tts/engine/mssapi5"
)

func TestMsSapi5Engine(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("MS SAPI5 is only available on Windows")
	}

	outputPath := enginetest.Configure(t)

	enginetest.RunInterfaceTests(t, enginetest.Case{
		Engine:     &mssapi5.MsSapi5{},
		EngineID:   "mssapi5",
		Model:      "mssapi5",
		OutputPath: outputPath,
		Character:  "Tester",
		NewPayload: func(voiceID string) []byte {
			payload, _ := json.Marshal(mssapi5.MsSapi5Request{
				Text:  "Testing one two three.",
				Voice: voiceID,
			})
			return payload
		},
	})
}
