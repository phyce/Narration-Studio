package player

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/flac"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/vorbis"
	"github.com/gopxl/beep/wav"
)

func PlayAudioFile(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(filePath))
	var streamer beep.StreamSeekCloser
	var format beep.Format

	switch ext {
	case ".wav":
		streamer, format, err = wav.Decode(file)
	case ".mp3":
		streamer, format, err = mp3.Decode(file)
	case ".ogg":
		streamer, format, err = vorbis.Decode(file)
	case ".flac":
		streamer, format, err = flac.Decode(file)
	default:
		return fmt.Errorf("unsupported format: %s (supported: .wav, .mp3, .ogg, .flac)", ext)
	}

	if err != nil {
		return fmt.Errorf("failed to decode: %w", err)
	}
	defer streamer.Close()

	targetRate := beep.SampleRate(48000)
	speaker.Init(targetRate, targetRate.N(time.Second/10))

	var finalStreamer beep.Streamer = streamer
	if format.SampleRate != targetRate {
		finalStreamer = beep.Resample(4, format.SampleRate, targetRate, streamer)
	}

	done := make(chan bool)
	speaker.Play(beep.Seq(finalStreamer, beep.Callback(func() {
		done <- true
	})))
	<-done

	return nil
}
