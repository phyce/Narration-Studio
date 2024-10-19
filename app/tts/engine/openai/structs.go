package openai

import "nstudio/app/tts/engine"

type Model struct {
	Voices []engine.Voice `json:"voice"`
}

type OpenAIRequest struct {
	Voice          string  `json:"voice"`
	Input          string  `json:"input"`
	Model          string  `json:"model"`
	ResponseFormat string  `json:"response_format"`
	Speed          float64 `json:"speed"`
}
