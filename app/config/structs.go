package config

import (
	"nstudio/app/enums/OutputType"
	"sync"
)

// <Config File>
type ConfigManager struct {
	//config map[string]Value `json:"config"`
	config   Base       `json:"config"`
	filePath string     `json:"filePath"`
	lock     sync.Mutex `json:"lock"`
}

type Base struct {
	Settings     Settings        `json:"settings"`
	Engine       Engine          `json:"engine"`
	ModelToggles map[string]bool `json:"modelToggles"`
	Info         Info            `json:"info"`
}

type Settings struct {
	OutputType OutputType.Option `json:"outputType"`
	OutputPath string            `json:"outputPath"`
	Debug      bool              `json:"debug"`
}

type Engine struct {
	Local Local `json:"local"`
	Api   Api   `json:"api"`
}

type Local struct {
	Piper Piper `json:"piper"`
}

type Piper struct {
	Path       string `json:"directory"`
	ModelsPath string `json:"modelsDirectory"`
}

type Api struct {
	OpenAI     OpenAI     `json:"openAI"`
	ElevenLabs ElevenLabs `json:"elevenLabs"`
}

type OpenAI struct {
	ApiKey     string `json:"apiKey"`
	OutputType string `json:"outputType"`
}

type ElevenLabs struct {
	ApiKey     string `json:"apiKey"`
	OutputType string `json:"outputType"`
}

type Info struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Website string `json:"website"`
}

//</Config File>
