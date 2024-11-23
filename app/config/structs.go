package config

import (
	"nstudio/app/enums/OutputType"
	"sync"
)

// <Config File>
type ConfigManager struct {
	//defaults map[string]Value `json:"defaults"`
	config   Base       `json:"defaults"`
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
	Piper   Piper   `json:"piper"`
	MsSapi4 MsSapi4 `json:"msSapi4"`
}

type Piper struct {
	Location        string `json:"location"`
	ModelsDirectory string `json:"modelsDirectory"`
}

type MsSapi4 struct {
	Location string `json:"location"`
	Pitch    int    `json:"pitch"`
	Speed    int    `json:"speed"`
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
	OS      string `json:"os"`
}

//</Config File>
