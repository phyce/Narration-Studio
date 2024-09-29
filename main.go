package main

import (
	"embed"
	"fmt"
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

	if err != nil {
		// TODO error popup or separate window before closing application
		fmt.Println("Failed to init config")
		fmt.Println(err)
		panic(err)
	}

	//TODO: Load Models from file
	piperEngine := engine.Engine{
		ID:     "piper",
		Name:   "Piper",
		Engine: &piper.Piper{},
		Models: map[string]engine.Model{
			"libritts": {
				ID:     "libritts",
				Name:   "LibriTTS",
				Engine: "piper",
				Download: engine.ModelDownload{
					Metadata: "",
					Model:    "https://mechanic.ink/narrator-studio/models/en/en_GB/vctk/medium/en_GB-vctk-medium.onnx",
					Phonemes: "https://mechanic.ink/narrator-studio/models/en/en_GB/vctk/medium/en_GB-vctk-medium.onnx.json",
				},
			},
			"vctk": {
				ID:     "vctk",
				Name:   "VCTK",
				Engine: "piper",
				Download: engine.ModelDownload{
					Metadata: "",
					Model:    "https://mechanic.ink/narrator-studio/models/en/en_GB/vctk/medium/en_GB-vctk-medium.onnx",
					Phonemes: "https://mechanic.ink/narrator-studio/models/en/en_GB/vctk/medium/en_GB-vctk-medium.onnx.json",
				},
			},
		},
	}
	voiceManager.GetInstance().RegisterEngine(piperEngine)

	err = wails.Run(&options.App{
		Title:            "Narrator Studio v0.8.0",
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
		// TODO error popup or separate window before closing application
		println("Error:", err.Error())
	}
}
