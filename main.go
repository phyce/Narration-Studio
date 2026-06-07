//go:build !cli && !clib

package main

import (
	"embed"
	"fmt"
	"nstudio/app/common/issue"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
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

	activityLogWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "activitylog",
		Title:            "Narration Studio - Activity Log",
		Width:            1000,
		Height:           700,
		Hidden:           true,
		URL:              "/activitylog.html",
		BackgroundColour: application.RGBA{Red: 27, Green: 38, Blue: 54, Alpha: 255},
	})
	activityLogWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		activityLogWindow.Hide()
	})

	if err := app.Run(); err != nil {
		issue.Panic("Failed to start app", err)
	}
}
