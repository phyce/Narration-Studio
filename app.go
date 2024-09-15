package main

import (
	"context"
	"nstudio/app/common/eventManager"
)

type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (app *App) startup(ctx context.Context) {
	app.ctx = ctx
	eventManager.GetInstance().Initialize(ctx)
}
