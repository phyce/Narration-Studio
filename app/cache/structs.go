package cache

import "sync"

type CharacterCache struct {
	Voice string            `json:"voice"` // Format: "engine:model:voiceID"
	Lines map[string]string `json:"lines"`
}

type ProfileCache struct {
	Characters map[string]*CharacterCache `json:"characters"`
	mutex      sync.RWMutex
}

type CacheManager struct {
	enabled  bool
	cacheDir string
	profiles map[string]*ProfileCache
	mutex    sync.RWMutex
}
