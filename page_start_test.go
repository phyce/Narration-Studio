//go:build !cli && !clib

package main

import "testing"

func TestStartPage(t *testing.T) {
	app := NewApp()

	t.Run("GetSettings", func(t *testing.T) {
		if app.GetSettings().Info.Name == "" {
			t.Error("GetSettings returned config with empty Info.Name")
		}
	})
}
