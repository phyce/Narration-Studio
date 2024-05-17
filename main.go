package main

import (
	"embed"
	"nstudio/app/config"
	"nstudio/app/tts/engine/piper"
	"nstudio/app/tts/voiceManager"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	err := config.GetInstance().Initialize()

	voiceManager.GetInstance().RegisterEngine("piper", &piper.Piper{})

	if err != nil {
		panic(err)
	}

	err = wails.Run(&options.App{
		Title:  "Narrator Studio v0.1.0",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
			//&config.Value{},
			//&config.ConfigManager{},
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
