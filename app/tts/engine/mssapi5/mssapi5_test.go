package mssapi5_test

import (
	"runtime"
	"testing"

	"nstudio/app/tts/engine/enginetest"
	"nstudio/app/tts/engine/mssapi5"
)

func TestMsSapi5Engine(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("MS SAPI5 is only available on Windows")
	}

	enginetest.Configure(t)

	enginetest.RunInterfaceTests(t, enginetest.Case{
		Engine:   &mssapi5.MsSapi5{},
		EngineID: "mssapi5",
		Model:    "mssapi5",
	})
}
