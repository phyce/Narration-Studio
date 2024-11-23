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
	"nstudio/app/tts/engine/mssapi4"
	"nstudio/app/tts/engine/openai"
	"nstudio/app/tts/engine/piper"
	"nstudio/app/tts/voiceManager"
	"runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	response.Initialize()

	err := config.Initialize(Info())
	if err != nil {
		issue.Panic("Failed to initialize defaults", err)
	}

	voiceManager.Initialize()

	registerEngines()

	app := NewApp()

	var startMode options.WindowStartState
	if config.Debug() {
		startMode = options.Minimised
	} else {
		startMode = options.Normal
	}

	err = wails.Run(&options.App{
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

func registerEngines() {
	//TODO: Load Models from file
	piperEngine := engine.Engine{
		ID:     "piper",
		Name:   "Piper",
		Engine: &piper.Piper{},
		Models: piper.FetchModels(),
	}
	err := voiceManager.RegisterEngine(piperEngine)
	if err != nil {
		issue.Trace(err)
	}

	openAIEngine := engine.Engine{
		ID:     "openai",
		Name:   "OpenAI",
		Engine: &openai.OpenAI{},
		Models: openai.FetchModels(),
	}
	err = voiceManager.RegisterEngine(openAIEngine)
	if err != nil {
		issue.Trace(err)
	}

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
	err = voiceManager.RegisterEngine(elevenLabsEngine)
	if err != nil {
		issue.Trace(err)
	}

	if runtime.GOOS == "windows" {
		msSapi4Engine := engine.Engine{
			ID:     "mssapi4",
			Name:   "Microsoft SAPI4",
			Engine: &mssapi4.MsSapi4{},
		}
		err = voiceManager.RegisterEngine(msSapi4Engine)
		if err != nil {
			issue.Trace(err)
		}
	}

	voiceManager.ReloadModels()
}
