package google

type GoogleRequest struct {
	Input       Input                `json:"input"`
	Voice       VoiceSelectionParams `json:"voice"`
	AudioConfig AudioConfig          `json:"audioConfig"`
}

type Input struct {
	Text string `json:"text"`
}

type VoiceSelectionParams struct {
	LanguageCode string `json:"languageCode"`
	Name         string `json:"name"`
	SsmlGender   string `json:"ssmlGender,omitempty"`
	ModelName    string `json:"modelName,omitempty"`
}

type AudioConfig struct {
	AudioEncoding string  `json:"audioEncoding"`
	SpeakingRate  float64 `json:"speakingRate"`
	Pitch         float64 `json:"pitch"`
}
