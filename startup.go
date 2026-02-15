package main

import (
	_ "embed"
	"nstudio/app/cache"
	"nstudio/app/common/response"
	"nstudio/app/common/status"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/engine/elevenlabs"
	"nstudio/app/tts/engine/mssapi4"
	"nstudio/app/tts/engine/openai"
	"nstudio/app/tts/engine/piper"
	"nstudio/app/tts/engine/google"
	"nstudio/app/tts/engine/gemini"
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
	status.Set(status.Loading, "Registering engines")
	//TODO: Load Models from file

	piperEngine := engine.Engine{
		ID:     "piper",
		Name:   "Piper",
		Type:   engine.Local,
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
		Type:   engine.Api,
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
		Type:   engine.Api,
		Engine: &elevenlabs.ElevenLabs{},
		Models: models,
	}

	err = modelManager.RegisterEngine(elevenLabsEngine)
	if err != nil {
		response.Err(err)
	}

	googleEngine := engine.Engine{
		ID:     "google",
		Name:   "Google Cloud",
		Type:   engine.Api,
		Engine: &google.Google{},
		Models: google.FetchModels(),
	}

	err = modelManager.RegisterEngine(googleEngine)
	if err != nil {
		response.Err(err)
	}

	geminiEngine := engine.Engine{
		ID:     "gemini",
		Name:   "Gemini",
		Type:   engine.Api,
		Engine: &gemini.Gemini{},
		Models: gemini.FetchModels(),
	}

	err = modelManager.RegisterEngine(geminiEngine)
	if err != nil {
		response.Err(err)
	}

	if runtime.GOOS == "windows" {
		msSapi4Engine := engine.Engine{
			ID:     "mssapi4",
			Name:   "Microsoft SAPI4",
			Type:   engine.Local,
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
