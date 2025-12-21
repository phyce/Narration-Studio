//go:build !cli

package main

import (
	"embed"
	"fmt"
	"nstudio/app/common/issue"
	"nstudio/app/common/response"
	"nstudio/app/config"
	"nstudio/app/tts/modelManager"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	SetupSignalHandler(func() {
		fmt.Println("\nShutting down...")
	})

	if err := initializeApp(""); err != nil {
		issue.Panic("Failed to initialize app", err)
	}

	modelManager.Initialize(true)

	registerEngines()

	app := NewApp()

	response.Initialize()

	var startMode options.WindowStartState
	if config.Debug() {
		startMode = options.Minimised
	} else {
		startMode = options.Normal
	}

	err := wails.Run(&options.App{
		Title:            config.GetInfo().Name + " v" + config.GetInfo().Version,
		Width:            1024,
		Height:           768,
		WindowStartState: startMode,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		issue.Panic("Failed to start app", err)
	}
}
