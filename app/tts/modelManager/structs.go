package modelManager

import (
	tts "nstudio/app/tts/engine"
	"sync"
)

type ModelPool struct {
	Instances []tts.Base
	pool      chan tts.Base // Channel-based pool for free instances
}

func (pool *ModelPool) GetNext() (tts.Base, bool) {
	if len(pool.Instances) == 0 {
		return nil, false
	}

	instance := <-pool.pool
	return instance, true
}

func (pool *ModelPool) Release(instance tts.Base) {
	pool.pool <- instance
}

type EngineEntry struct {
	Engine tts.Engine
	Models map[string]*ModelPool
}

func (entry *EngineEntry) GetModelInstance(modelID string) (tts.Base, *ModelPool, bool) {
	pool, exists := entry.Models[modelID]
	if !exists {
		return nil, nil, false
	}

	instance, ok := pool.GetNext()
	if !ok {
		return nil, nil, false
	}

	return instance, pool, true
}

type modelManager struct {
	sync.RWMutex
	Engines   map[string]*EngineEntry
	IsGUIMode bool
}
