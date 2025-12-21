package main

import (
	"context"
	"nstudio/app/common/eventManager"
	"os"
	"os/exec"
	"runtime"
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
}

func clearConsole() error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
