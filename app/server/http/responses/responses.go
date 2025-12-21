package responses

type TTSRequest struct {
	Text    string                 `json:"text" validate:"required,min=1,max=10000"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type TTSResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	AudioData []byte `json:"audio_data,omitempty"`
	Format    string `json:"format,omitempty"`
	Duration  string `json:"duration,omitempty"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Code    int    `json:"code"`
}

type HealthResponse struct {
	Status            string             `json:"status"`
	Version           string             `json:"version"`
	Uptime            string             `json:"uptime"`
	EnabledEngines    []EngineHealthInfo `json:"enabled_engines"`
	TotalModels       int                `json:"total_models"`
	TotalVoices       int                `json:"total_voices"`
	ProcessedMessages int64              `json:"processed_messages"`
}

type EngineHealthInfo struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	EnabledCount int         `json:"enabled_models"`
	TotalCount   int         `json:"total_models"`
	VoiceCount   int         `json:"voice_count"`
	Instances    int         `json:"instances"`
	Models       []ModelInfo `json:"models"`
}

type ModelInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Instances int    `json:"instances"`
}
