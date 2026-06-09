//go:build !cli && !clib

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"nstudio/app/common/response"
	"nstudio/app/config"
	"nstudio/app/tts/modelManager"

	"github.com/charmbracelet/log"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.FatalLevel)

	tempDir, err := os.MkdirTemp("", "nstudio-uitest")
	if err != nil {
		panic(err)
	}

	if err := initializeApp(filepath.Join(tempDir, "config.json")); err != nil {
		panic(err)
	}

	base := config.Get()
	base.ModelToggles = map[string]bool{}
	_ = config.Set(base)

	modelManager.Initialize(true)
	registerEngines()
	response.Initialize()

	code := m.Run()

	_ = os.RemoveAll(tempDir)
	os.Exit(code)
}

func mustParseJSON(t *testing.T, label, s string) interface{} {
	t.Helper()
	var value interface{}
	if err := json.Unmarshal([]byte(s), &value); err != nil {
		t.Fatalf("%s returned invalid JSON: %v (%.120q)", label, err, s)
	}
	return value
}