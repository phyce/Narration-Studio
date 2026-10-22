//go:build !cli && !clib

package main

import "testing"

func TestVoicePacksPage(t *testing.T) {
	app := NewApp()

	t.Run("GetAvailableModels", func(t *testing.T) {
		value := mustParseJSON(t, "GetAvailableModels", app.GetAvailableModels())
		if _, ok := value.(map[string]interface{}); !ok {
			t.Error("GetAvailableModels did not return a JSON object")
		}
	})

	t.Run("RefreshModels", func(t *testing.T) {
		app.RefreshModels()
	})

	t.Run("ReloadVoicePacks", func(t *testing.T) {
		app.ReloadVoicePacks()
	})

	t.Run("EnginePackFetchAvailable", func(t *testing.T) {
		value := mustParseJSON(t, "EnginePackFetchAvailable", app.EnginePackFetchAvailable())
		if value != nil {
			if _, ok := value.([]interface{}); !ok {
				t.Errorf("EnginePackFetchAvailable returned non-array JSON: %T", value)
			}
		}
	})
}
