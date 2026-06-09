//go:build !cli && !clib

package main

import (
	"encoding/json"
	"testing"
)

func TestSettingsPage(t *testing.T) {
	app := NewApp()

	t.Run("GetSettings", func(t *testing.T) {
		settings := app.GetSettings()
		if settings.Info.Name == "" {
			t.Error("GetSettings returned config with empty Info.Name")
		}
	})

	t.Run("GetConfigSchema", func(t *testing.T) {
		schema := app.GetConfigSchema()
		if schema == "" || schema == "{}" || schema == "[]" {
			t.Fatalf("GetConfigSchema returned empty schema: %q", schema)
		}
		var parsed interface{}
		if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
			t.Errorf("GetConfigSchema returned invalid JSON: %v", err)
		}
	})

	t.Run("SaveSettings", func(t *testing.T) {
		settings := app.GetSettings()
		const marker = "ui-test-output-path"
		settings.Settings.OutputPath = marker

		app.SaveSettings(settings)

		reloaded := app.GetSettings()
		if reloaded.Settings.OutputPath != marker {
			t.Errorf("SaveSettings did not persist OutputPath: got %q, want %q", reloaded.Settings.OutputPath, marker)
		}
	})
}
