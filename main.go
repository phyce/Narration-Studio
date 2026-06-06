//go:build !cli && !clib

package main

import (
	"embed"
	"fmt"
	"nstudio/app/common/issue"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	SetupSignalHandler(func() {
		fmt.Println("\nShutting down...")
	})

	app := application.New(application.Options{
		Name: "Narration Studio",
		Services: []application.Service{
			application.NewService(NewApp()),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "Narration Studio",
		Width:            1024,
		Height:           768,
		BackgroundColour: application.RGBA{Red: 27, Green: 38, Blue: 54, Alpha: 255},
	})

	if err := app.Run(); err != nil {
		issue.Panic("Failed to start app", err)
	}
}
