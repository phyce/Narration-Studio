//go:build !cli && !clib

package main

import (
	"context"
	"nstudio/app/common/issue"
	"nstudio/app/common/response"
	"nstudio/app/common/status"
	"nstudio/app/tts/modelManager"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type App struct{}

func NewApp() *App {
	return &App{}
}

func (app *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	if err := initializeApp(""); err != nil {
		issue.Panic("Failed to initialize app", err)
	}

	modelManager.Initialize(true)
	registerEngines()
	response.Initialize()
	status.Set(status.Ready, "")
	return nil
}

func clearConsole() error {
	_, err := os.Stdout.WriteString("\033[2J\033[H")
	return err
}
