package modelManager

import (
	tts "nstudio/app/tts/engine"
	"sync"
	"time"
)

type ModelPool struct {
	Instances []tts.Base
	pool      chan tts.Base
	detached  chan struct{}
}

func newModelPool(instances []tts.Base) *ModelPool {
	poolChannel := make(chan tts.Base, len(instances))
	for _, instance := range instances {
		poolChannel <- instance
	}

	return &ModelPool{
		Instances: instances,
		pool:      poolChannel,
		detached:  make(chan struct{}),
	}
}

func (pool *ModelPool) GetNext() (tts.Base, bool) {
	if len(pool.Instances) == 0 {
		return nil, false
	}

	select {
	case <-pool.detached:
		return nil, false
	default:
	}

	select {
	case instance := <-pool.pool:
		return instance, true
	case <-pool.detached:
		return nil, false
	}
}

func (pool *ModelPool) Release(instance tts.Base) {
	pool.pool <- instance
}

func (pool *ModelPool) detach() {
	close(pool.detached)
}

func (pool *ModelPool) drain(timeout time.Duration) []tts.Base {
	recovered := make([]tts.Base, 0, len(pool.Instances))
	deadline := time.After(timeout)

	for range pool.Instances {
		select {
		case instance := <-pool.pool:
			recovered = append(recovered, instance)
		case <-deadline:
			return recovered
		}
	}

	return recovered
}

type EngineEntry struct {
	Engine tts.Engine
	Models map[string]*ModelPool
}

type modelManager struct {
	sync.RWMutex
	Engines   map[string]*EngineEntry
	IsGUIMode bool
	released  bool
}

type ApiAvailability struct {
	Available     bool   `json:"available"`
	MissingApiKey bool   `json:"missingApiKey"`
	Reason        string `json:"reason,omitempty"`
}
