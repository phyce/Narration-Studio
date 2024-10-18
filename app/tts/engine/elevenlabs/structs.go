package elevenlabs

import "nstudio/app/tts/engine"

type VoicesResponse struct {
	Voices []VoiceDetail `json:"voices"`
}

type VoiceDetail struct {
	VoiceID string `json:"voice_id"`
	Name    string `json:"name"`
	Labels  Labels `json:"labels"`
}

type Labels struct {
	Gender string `json:"gender"`
}

type ModelResponse struct {
	ModelID string `json:"model_id"`
	Name    string `json:"name"`
}

type Model struct {
	Voices []engine.Voice `json:"voice"`
}

type ElevenLabsRequest struct {
	Text          string        `json:"text"`
	ModelID       string        `json:"model_id"`
	VoiceSettings VoiceSettings `json:"voice_settings"`
}

type VoiceSettings struct {
	Stability       float32 `json:"stability"`
	SimilarityBoost float32 `json:"similarity_boost"`
}
