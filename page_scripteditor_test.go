//go:build !cli && !clib

package main

import "testing"

func TestScriptEditorPage(t *testing.T) {
	app := NewApp()

	t.Run("GetSettings", func(t *testing.T) {
		if app.GetSettings().Info.Name == "" {
			t.Error("GetSettings returned config with empty Info.Name")
		}
	})

	t.Run("SaveSettings", func(t *testing.T) {
		settings := app.GetSettings()
		const marker = "script-editor-output-path"
		settings.Settings.OutputPath = marker
		app.SaveSettings(settings)
		if app.GetSettings().Settings.OutputPath != marker {
			t.Errorf("SaveSettings did not persist OutputPath")
		}
	})
}
