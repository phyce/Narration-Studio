package native

import (
	"nstudio/app/tts/engine"
	"sync"
)

type PiperNativeInstance struct {
	synth  *Synthesizer
	mu     sync.Mutex
	Voices []engine.Voice
}

type PiperInput struct {
	Text      string `json:"text"`
	SpeakerID int    `json:"speaker_id"`
}

type Piper struct {
	models        map[string]*PiperNativeInstance
	espeakDataDir string
	modelsDir     string
	useGPU        bool
}
