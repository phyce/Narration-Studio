package main

import (
	_ "embed"
	"nstudio/app/cache"
	"nstudio/app/common/response"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/engine/elevenlabs"
	"nstudio/app/tts/engine/mssapi4"
	"nstudio/app/tts/engine/openai"
	"nstudio/app/tts/engine/piper"
	"nstudio/app/tts/modelManager"
	"nstudio/app/tts/profile"
	"path/filepath"
	"runtime"
)

//go:embed usage.txt
var helpText string

func initializeApp(configFile string) error {
	var err error
	if configFile != "" {
		err = config.InitializeWithPath(Info(), configFile)
	} else {
		err = config.Initialize(Info())
	}
	if err != nil {
		return response.Err(err)
	}

	profileDir := filepath.Join(config.GetCurrentConfigPath(), "profiles")
	if err := profile.InitializeProfileDirectory(profileDir); err != nil {
		return response.Err(err)
	}

	if err := cache.Initialize(); err != nil {
		return response.Err(err)
	}

	return nil
}

func registerEngines() {
	//TODO: Load Models from file

	piperEngine := engine.Engine{
		ID:     "piper",
		Name:   "Piper",
		Engine: &piper.Piper{},
		Models: piper.FetchModels(),
	}

	err := modelManager.RegisterEngine(piperEngine)
	if err != nil {
		response.Err(err)
	}

	openAIEngine := engine.Engine{
		ID:     "openai",
		Name:   "OpenAI",
		Engine: &openai.OpenAI{},
		Models: openai.FetchModels(),
	}

	err = modelManager.RegisterEngine(openAIEngine)
	if err != nil {
		response.Err(err)
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

	err = modelManager.RegisterEngine(elevenLabsEngine)
	if err != nil {
		response.Err(err)
	}

	if runtime.GOOS == "windows" {
		msSapi4Engine := engine.Engine{
			ID:     "mssapi4",
			Name:   "Microsoft SAPI4",
			Engine: &mssapi4.MsSapi4{},
			Models: mssapi4.FetchModels(),
		}

		err = modelManager.RegisterEngine(msSapi4Engine)
		if err != nil {
			response.Err(err)
		}
	}

	modelManager.ReloadModels()
}
