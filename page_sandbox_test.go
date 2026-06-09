//go:build !cli && !clib

package main

import (
	"runtime"
	"testing"
)

func TestSandboxPage(t *testing.T) {
	app := NewApp()

	t.Run("GetEngines", func(t *testing.T) {
		result := app.GetEngines()
		value := mustParseJSON(t, "GetEngines", result)
		engines, ok := value.([]interface{})
		if !ok {
			t.Fatalf("GetEngines did not return a JSON array: %.120q", result)
		}
		if len(engines) == 0 {
			t.Error("GetEngines returned no engines")
		}
		for index, raw := range engines {
			engine, ok := raw.(map[string]interface{})
			if !ok {
				t.Errorf("engine %d is not an object", index)
				continue
			}
			if id, _ := engine["id"].(string); id == "" {
				t.Errorf("engine %d has empty id", index)
			}
			if name, _ := engine["name"].(string); name == "" {
				t.Errorf("engine %d has empty name", index)
			}
		}
	})

	t.Run("GetModelVoices", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("using the SAPI5 model for voices, which is Windows-only")
		}
		result := app.GetModelVoices("mssapi5", "mssapi5")
		value := mustParseJSON(t, "GetModelVoices", result)
		voices, ok := value.([]interface{})
		if !ok {
			t.Fatalf("GetModelVoices did not return a JSON array: %.120q", result)
		}
		if len(voices) == 0 {
			t.Error("GetModelVoices returned no voices for mssapi5")
		}
	})
}
