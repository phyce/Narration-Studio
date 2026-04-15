//go:build !windows

package mssapi5

import "nstudio/app/tts/engine"

func initCOM() error {
	return nil
}

func enumerateVoices() ([]engine.Voice, error) {
	return nil, nil
}

func synthesize(voiceID string, text string, rate int, volume int) ([]byte, error) {
	return nil, nil
}
