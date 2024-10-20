package main

import (
	"embed"
	"fmt"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"nstudio/app/common/response"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/engine/elevenlabs"
	"nstudio/app/tts/engine/openai"
	"nstudio/app/tts/engine/piper"
	"nstudio/app/tts/voiceManager"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	err := config.GetInstance().Initialize()
	fmt.Println()
	response.Initialize()

	if err != nil {
		// TODO error popup or separate window before closing application
		fmt.Println("Failed to init config")
		fmt.Println(err)
		panic(err)
	}

	//TODO: Load Models from file
	piper := engine.Engine{
		ID:     "piper",
		Name:   "Piper",
		Engine: &piper.Piper{},
		Models: piper.FetchModels(),
	}
	voiceManager.GetInstance().RegisterEngine(piper)

	openAI := engine.Engine{
		ID:     "openai",
		Name:   "OpenAI",
		Engine: &openai.OpenAI{},
		Models: openai.FetchModels(),
	}
	voiceManager.GetInstance().RegisterEngine(openAI)

	models, err := elevenlabs.FetchModels()
	if err != nil {
		models = make(map[string]engine.Model)
	}

	elevenLabs := engine.Engine{
		ID:     "elevenlabs",
		Name:   "ElevenLabs",
		Engine: &elevenlabs.ElevenLabs{},
		Models: models,
	}
	voiceManager.GetInstance().RegisterEngine(elevenLabs)

	err = wails.Run(&options.App{
		Title:            "Narrator Studio v0.11.0",
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
