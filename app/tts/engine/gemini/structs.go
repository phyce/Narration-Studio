package gemini

type GeminiRequest struct {
	Model            string           `json:"model,omitempty"`
	Contents         []Content        `json:"contents"`
	GenerationConfig GenerationConfig `json:"generationConfig"`
}

type Content struct {
	Parts []Part `json:"parts"`
}

type Part struct {
	Text string `json:"text"`
}

type GenerationConfig struct {
	ResponseModalities []string     `json:"responseModalities"`
	SpeechConfig       SpeechConfig `json:"speechConfig"`
}

type SpeechConfig struct {
	VoiceConfig VoiceConfig `json:"voiceConfig"`
}

type VoiceConfig struct {
	PrebuiltVoiceConfig PrebuiltVoiceConfig `json:"prebuiltVoiceConfig"`
}

type PrebuiltVoiceConfig struct {
	VoiceName string `json:"voiceName"`
}

type GeminiResponse struct {
	Candidates []Candidate `json:"candidates"`
}

type Candidate struct {
	Content ContentResponse `json:"content"`
}

type ContentResponse struct {
	Parts []PartResponse `json:"parts"`
}

type PartResponse struct {
	InlineData InlineData `json:"inlineData"`
}

type InlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // Base64 encoded
}
