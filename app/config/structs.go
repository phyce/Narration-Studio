package config

import (
	"encoding/json"
	"nstudio/app/enums/OutputType"
	"sync"
)

// <Old>
type Value struct {
	String *string `json:"string"`
	Int    *int    `json:"int"`
	Raw    string  `json:"raw"`
}

func (cv Value) MarshalJSON() ([]byte, error) {
	if cv.String != nil {
		return json.Marshal(cv.String)
	}
	if cv.Raw != "" {
		return json.Marshal(cv.Raw)
	}
	return json.Marshal(cv.Int)
}

func (cv *Value) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	if data[0] == '"' {
		return json.Unmarshal(data, &cv.String)
	}
	if data[0] == '{' || data[0] == '[' {
		cv.Raw = string(data)
		return nil
	}
	return json.Unmarshal(data, &cv.Int)
}

type ConfigValueInt struct {
	Value uint   `json:"value"`
	Name  string `json:"name"`
	Label string `json:"label"`
}

type ConfigValueString struct {
	Value string `json:"value"`
	Name  string `json:"name"`
	Label string `json:"label"`
}

//</Old>

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
	Info         Info
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
	Title   string `json:"name"`
	Version string `json:"version"`
}

//</Config File>
