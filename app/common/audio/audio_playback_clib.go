//go:build clib

package audio

import (
	"context"

	"nstudio/app/common/response"
)

// Playback functions are not available in DLL mode.
// The host application handles audio playback.

func PlayStream(ctx context.Context, stream *Stream) error {
	stream.Close()
	return response.Err("audio playback not available in DLL mode")
}

func SetStreamPaused(paused bool) {
}

func PlayPCMAudioBytes(audioClip []byte) error {
	return response.Err("audio playback not available in DLL mode")
}

func PlayRawAudioBytes(audioClip []byte) {
}

func PlayRawAudioBytesAt(audioClip []byte, sourceSampleRate int) {
}

func PlayWAVBytes(wavData []byte) error {
	return response.Err("audio playback not available in DLL mode")
}

func PlayFLACAudioBytes(audioClip []byte) error {
	return response.Err("audio playback not available in DLL mode")
}

func PlayMP3AudioBytes(audioClip []byte) error {
	return response.Err("audio playback not available in DLL mode")
}

func SaveFLACAsWAV(flacAudioClip []byte, filename string) error {
	return response.Err("FLAC to WAV conversion not available in DLL mode")
}
