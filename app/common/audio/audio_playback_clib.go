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
