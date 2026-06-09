//go:build !cli && !clib

package main

import "testing"

func TestServerPage(t *testing.T) {
	app := NewApp()

	t.Run("GetServerStatus", func(t *testing.T) {
		mustParseJSON(t, "GetServerStatus", app.GetServerStatus())
	})

	t.Run("GetServerLogs", func(t *testing.T) {
		mustParseJSON(t, "GetServerLogs", app.GetServerLogs())
	})
}
