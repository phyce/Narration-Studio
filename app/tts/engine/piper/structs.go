package piper

import (
	"io"
	"nstudio/app/tts/engine"
	"os/exec"
	"sync"
)

type PiperInstance struct {
	command   *exec.Cmd
	stdin     io.WriteCloser
	stderr    io.ReadCloser
	stdout    io.ReadCloser
	audioData *AudioBuffer
	Voices    []engine.Voice
}

type PiperInput struct {
	Text       string `json:"text"`
	SpeakerID  int    `json:"speaker_id"`
	OutputFile string `json:"output_file"`
}

type PiperInputLite struct {
	Text      string `json:"text"`
	SpeakerID int    `json:"speaker_id"`
}

type Piper struct {
	models    map[string]PiperInstance
	piperPath string
	modelPath string
	initOnce  sync.Once
}
