//go:build clib

package audio

import (
	"context"

	"nstudio/app/common/response"
)

func PlayStream(ctx context.Context, stream *Stream) error {
	stream.Close()
	return response.Err("audio playback not available in DLL mode")
}

func SetStreamPaused(paused bool) {
}
