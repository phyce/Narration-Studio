package audio

import (
	"strings"
)

// GetContentType returns the MIME content type for the given audio format
func GetContentType(format string) string {
	switch strings.ToLower(format) {
	case "wav":
		return "audio/wav"
	case "flac":
		return "audio/flac"
	case "ogg":
		return "audio/ogg"
	case "mp3":
		return "audio/mpeg"
	case "pcm", "pcm_s16le":
		return "audio/pcm"
	default:
		return "audio/wav"
	}
}
