//go:build !cli

package main

import (
	"embed"
	"fmt"
	"nstudio/app/common/issue"

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

	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "Narration Studio",
		Width:  1024,
		Height: 768,
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
