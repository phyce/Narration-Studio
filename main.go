package main

import (
	"embed"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/engine/piper"
	"nstudio/app/tts/voiceManager"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	err := config.GetInstance().Initialize()

	//TODO: Load Models from file
	piperEngine := engine.Engine{
		ID:     "piper",
		Name:   "Piper",
		Engine: &piper.Piper{},
		Models: map[string]engine.Model{
			"libritts": {ID: "libritts", Name: "LibriTTS"},
			"vctk":     {ID: "vctk", Name: "VCTK"},
		},
	}

	voiceManager.GetInstance().RegisterEngine(piperEngine)

	if err != nil {
		panic(err)
	}

	err = wails.Run(&options.App{
		Title:            "Narrator Studio v0.2.0",
		Width:            1024,
		Height:           768,
		WindowStartState: options.Minimised,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
			//&engine.Model{},
			//&engine.Voice{},
			//&config.Value{},
			//&config.ConfigManager{},
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
