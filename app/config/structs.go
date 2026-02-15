package config

import (
	"nstudio/app/enums/OutputType"
	"sync"
)

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
	OutputType OutputType.Option  `json:"outputType"`
	OutputPath string             `json:"outputPath"`
	Debug      bool               `json:"debug"`
	AudioCache AudioCacheSettings `json:"audioCache,omitempty"`
	Server     ServerSettings     `json:"server,omitempty"`
}

type AudioCacheSettings struct {
	Enabled  bool   `json:"enabled"`
	Location string `json:"location"`
}

type ServerSettings struct {
	Auth    AuthSettings          `json:"auth,omitempty"`
	Engines ServerSettingsEngines `json:"engines, omitempty"`
}
type ServerSettingsEngines struct {
	Piper      map[string]ModelInstances `json:"piper,omitempty"`
	OpenAI     map[string]ModelInstances `json:"openai,omitempty"`
	ElevenLabs map[string]ModelInstances `json:"elevenlabs,omitempty"`
	MsSapi4    map[string]ModelInstances `json:"mssapi4,omitempty"`
	Google     map[string]ModelInstances `json:"google,omitempty"`
	Gemini     map[string]ModelInstances `json:"gemini,omitempty"`
}

type AuthSettings struct {
	Key      string `json:"key"`
	AdminKey string `json:"adminKey"`
}

type ModelInstances struct {
	Instances int `json:"instances"`
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
	Google     Google     `json:"google"`
	Gemini     Gemini     `json:"gemini"`
}

type OpenAI struct {
	ApiKey     string `json:"apiKey"`
	OutputType string `json:"outputType"`
}

type ElevenLabs struct {
	ApiKey     string `json:"apiKey"`
	OutputType string `json:"outputType"`
}

type Google struct {
	ApiKey string `json:"apiKey"`
}

type Gemini struct {
	ApiKey string `json:"apiKey"`
}

type Info struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Website string `json:"website"`
	OS      string `json:"os"`
}
