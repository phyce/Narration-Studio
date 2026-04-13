//go:build clib

package audio

import "fmt"

// Playback functions are not available in DLL mode.
// The host application handles audio playback.

func PlayPCMAudioBytes(audioClip []byte) error {
	return fmt.Errorf("audio playback not available in DLL mode")
}

func PlayRawAudioBytes(audioClip []byte) {
}

func PlayFLACAudioBytes(audioClip []byte) error {
	return fmt.Errorf("audio playback not available in DLL mode")
}

func PlayMP3AudioBytes(audioClip []byte) error {
	return fmt.Errorf("audio playback not available in DLL mode")
}

func SaveFLACAsWAV(flacAudioClip []byte, filename string) error {
	return fmt.Errorf("FLAC to WAV conversion not available in DLL mode")
}
