package profile

import (
	"nstudio/app/common/util"
	"sync"
)

type ProfileSettings struct {
	ModelToggles map[string]bool `json:"modelToggles,omitempty"`
	CacheEnabled *bool           `json:"cacheEnabled,omitempty"` // nil = use global
}

type Profile struct {
	ID          string                          `json:"id"`
	Name        string                          `json:"name"`
	Description string                          `json:"description"`
	CreatedAt   string                          `json:"created_at"`
	UpdatedAt   string                          `json:"updated_at"`
	Voices      map[string]*util.CharacterVoice `json:"voices"`
	Settings    *ProfileSettings                `json:"settings,omitempty"`
	mutex       sync.RWMutex                    `json:"-"`
}

type ProfileMetadata struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	VoiceCount  int    `json:"voice_count"`
}

type ProfileManager struct {
	cache map[string]*Profile
	mutex sync.RWMutex
}
