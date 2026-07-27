package audio

import (
	"encoding/binary"
	"strings"
)

// StreamingWAVHeader builds a 44-byte PCM WAV header for a stream of unknown
// length, using the 0xFFFFFFFF RIFF/data size convention players treat as
// "read until the stream ends".
func StreamingWAVHeader(format StreamFormat) []byte {
	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], 0xFFFFFFFF)
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], uint16(format.Channels))
	binary.LittleEndian.PutUint32(header[24:28], uint32(format.SampleRate))
	byteRate := format.SampleRate * format.Channels * format.BitDepth / 8
	binary.LittleEndian.PutUint32(header[28:32], uint32(byteRate))
	blockAlign := format.Channels * format.BitDepth / 8
	binary.LittleEndian.PutUint16(header[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(header[34:36], uint16(format.BitDepth))
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], 0xFFFFFFFF)
	return header
}

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
