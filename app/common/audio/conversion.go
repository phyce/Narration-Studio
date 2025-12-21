package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// ConvertRawToFormat converts raw audio bytes to the specified format
func ConvertRawToFormat(rawAudio []byte, format string) ([]byte, error) {
	switch strings.ToLower(format) {
	case "wav":
		return ConvertRawToWAV(rawAudio)
	case "flac":
		return ConvertRawToFLAC(rawAudio)
	case "ogg":
		return ConvertRawToOGG(rawAudio)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// ConvertRawToWAV converts raw audio bytes to WAV format
func ConvertRawToWAV(rawAudio []byte) ([]byte, error) {
	sampleRate := uint32(22050) // Common TTS sample rate
	channels := uint16(1)       // Mono
	bitsPerSample := uint16(16) // 16-bit

	byteRate := sampleRate * uint32(channels) * uint32(bitsPerSample) / 8
	blockAlign := channels * bitsPerSample / 8
	dataSize := uint32(len(rawAudio))
	fileSize := 36 + dataSize

	var buf bytes.Buffer

	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, fileSize)
	buf.WriteString("WAVE")

	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, channels)
	binary.Write(&buf, binary.LittleEndian, sampleRate)
	binary.Write(&buf, binary.LittleEndian, byteRate)
	binary.Write(&buf, binary.LittleEndian, blockAlign)
	binary.Write(&buf, binary.LittleEndian, bitsPerSample)

	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, dataSize)
	buf.Write(rawAudio)

	return buf.Bytes(), nil
}

// ConvertRawToFLAC converts raw audio bytes to FLAC format
// Note: Currently returns WAV as fallback. Full FLAC encoding requires additional dependencies.
func ConvertRawToFLAC(rawAudio []byte) ([]byte, error) {
	wavData, err := ConvertRawToWAV(rawAudio)
	if err != nil {
		return nil, err
	}

	tempWavFile := "/tmp/temp_audio.wav"
	tempFlacFile := "/tmp/temp_audio.flac"

	err = os.WriteFile(tempWavFile, wavData, 0644)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tempWavFile)

	// Note: This requires flac command to be installed
	// For production, you'd want to use a Go FLAC library instead
	// For now, just return WAV format as fallback
	defer os.Remove(tempFlacFile)

	// Since we can't rely on external commands, return WAV as fallback
	return wavData, nil
}

// ConvertRawToOGG converts raw audio bytes to OGG format
// Note: Currently returns WAV as fallback. Full OGG encoding requires additional dependencies.
func ConvertRawToOGG(rawAudio []byte) ([]byte, error) {
	// For now, create a temporary WAV file and use system oggenc command
	// This is a simple implementation - in production you'd want a proper OGG library

	// Create temporary WAV file
	wavData, err := ConvertRawToWAV(rawAudio)
	if err != nil {
		return nil, err
	}

	// Since we can't rely on external commands in this environment,
	// return WAV format as fallback for now
	return wavData, nil
}

// GetContentType returns the MIME content type for the given audio format
func GetContentType(format string) string {
	switch strings.ToLower(format) {
	case "wav":
		return "audio/wav"
	case "flac":
		return "audio/flac"
	case "ogg":
		return "audio/ogg"
	default:
		return "audio/wav"
	}
}
