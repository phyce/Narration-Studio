//go:build !windows && !linux

package native

import "fmt"

type Synthesizer struct{}

type SynthesizeOptions struct {
	SpeakerID   int
	LengthScale float32
	NoiseScale  float32
	NoiseWScale float32
}

func IsGPUAvailable() bool {
	return false
}

func IsNativeAvailable() bool {
	return false
}

func NewSynthesizer(modelPath, configPath, espeakDataPath string, useGPU bool) (*Synthesizer, error) {
	return nil, fmt.Errorf("piper-native is only supported on Windows")
}

func (s *Synthesizer) Free() {}

func (s *Synthesizer) DefaultOptions() SynthesizeOptions {
	return SynthesizeOptions{}
}

func (s *Synthesizer) Synthesize(text string, opts *SynthesizeOptions) ([]byte, int, error) {
	return nil, 0, fmt.Errorf("piper-native is only supported on Windows")
}
