package main

import (
	"context"
	"nstudio/app/common/eventManager"
	"nstudio/app/common/issue"
	"nstudio/app/common/response"
	"nstudio/app/common/status"
	"nstudio/app/tts/modelManager"
	"os"
)

type App struct {
	context context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (app *App) startup(ctx context.Context) {
	app.context = ctx
	eventManager.GetInstance().Initialize(ctx)

	if err := initializeApp(""); err != nil {
		issue.Panic("Failed to initialize app", err)
	}

	modelManager.Initialize(true)
	registerEngines()
	response.Initialize()
	status.Set(status.Ready, "")
}

func clearConsole() error {
	_, err := os.Stdout.WriteString("\033[2J\033[H")
	return err
}
