package main

import (
	"embed"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"nstudio/app/common/issue"
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
	response.Initialize()

	err := config.Initialize(Info())
	if err != nil {
		issue.Panic("Failed to initialize config", err)
	}

	voiceManager.Initialize()

	registerEngines()

	app := NewApp()

	err = wails.Run(&options.App{
		Title:            config.GetInfo().Title + " v" + config.GetInfo().Version,
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
		},
	})

	if err != nil {
		issue.Panic("Failed to start app", err)
	}
}

func registerEngines() {
	//TODO: Load Models from file
	piperEngine := engine.Engine{
		ID:     "piper",
		Name:   "Piper",
		Engine: &piper.Piper{},
		Models: piper.FetchModels(),
	}
	voiceManager.RegisterEngine(piperEngine)

	openAIEngine := engine.Engine{
		ID:     "openai",
		Name:   "OpenAI",
		Engine: &openai.OpenAI{},
		Models: openai.FetchModels(),
	}
	voiceManager.RegisterEngine(openAIEngine)

	models, err := elevenlabs.FetchModels()
	if err != nil {
		models = make(map[string]engine.Model)
	}

	elevenLabsEngine := engine.Engine{
		ID:     "elevenlabs",
		Name:   "ElevenLabs",
		Engine: &elevenlabs.ElevenLabs{},
		Models: models,
	}
	voiceManager.RegisterEngine(elevenLabsEngine)
}
