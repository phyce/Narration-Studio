package http

type ProfileTTSRequest struct {
	Profile   string                 `json:"profile" validate:"required"`
	Character string                 `json:"character" validate:"required"`
	Text      string                 `json:"text" validate:"required,min=1,max=10000"`
	Options   map[string]interface{} `json:"options,omitempty"`
}

type SimpleTTSRequest struct {
	Text    string                 `json:"text" validate:"required,min=1,max=10000"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type ProfileCreateRequest struct {
	ID          string `json:"id" validate:"required"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
