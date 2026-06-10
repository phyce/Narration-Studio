//go:build !clib

package audio

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/speaker"
)

var (
	testSpeakerOnce sync.Once
	testSpeakerErr  error
)

func initTestSpeaker(t *testing.T) {
	t.Helper()
	testSpeakerOnce.Do(func() {
		sampleRate := beep.SampleRate(playbackTargetRate)
		testSpeakerErr = speaker.Init(sampleRate, sampleRate.N(time.Second/10))
	})
	if testSpeakerErr != nil {
		t.Skipf("no audio device available: %v", testSpeakerErr)
	}
}

func silentPCM(format StreamFormat, duration time.Duration) []byte {
	return make([]byte, int(time.Duration(format.SampleRate*format.Channels*2)*duration/time.Second))
}

func TestPlayStream(t *testing.T) {
	initTestSpeaker(t)
	format := StreamFormat{SampleRate: 24000, Channels: 1, BitDepth: 16}

	t.Run("BufferedStreamPlaysToCompletion", func(t *testing.T) {
		stream := NewBufferedStream(format, silentPCM(format, 300*time.Millisecond))
		if err := PlayStream(context.Background(), stream); err != nil {
			t.Fatalf("PlayStream: %v", err)
		}
	})

	t.Run("LiveStreamSurvivesUnderrun", func(t *testing.T) {
		stream, writer := NewPipeStream(format)
		go func() {
			writer.Write(silentPCM(format, 100*time.Millisecond))
			time.Sleep(250 * time.Millisecond)
			writer.Write(silentPCM(format, 100*time.Millisecond))
			writer.Close()
		}()
		if err := PlayStream(context.Background(), stream); err != nil {
			t.Fatalf("PlayStream: %v", err)
		}
	})

	t.Run("ProducerErrorSurfacesAfterPlayout", func(t *testing.T) {
		stream, writer := NewPipeStream(format)
		producerError := errors.New("synthesis failed")
		go func() {
			writer.Write(silentPCM(format, 100*time.Millisecond))
			writer.CloseWithError(producerError)
		}()
		if err := PlayStream(context.Background(), stream); !errors.Is(err, producerError) {
			t.Fatalf("got %v, want producer error", err)
		}
	})

	t.Run("CancelStopsPlaybackPromptly", func(t *testing.T) {
		stream, writer := NewPipeStream(format)
		writer.Write(silentPCM(format, 2*time.Second))

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(150 * time.Millisecond)
			cancel()
		}()

		started := time.Now()
		if err := PlayStream(ctx, stream); !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want context.Canceled", err)
		}
		if elapsed := time.Since(started); elapsed > time.Second {
			t.Fatalf("cancel took %v, want well under the clip duration", elapsed)
		}

		if _, err := writer.Write([]byte{0, 0}); err != ErrStreamClosed {
			t.Fatalf("got %v, want ErrStreamClosed after abandoned playback", err)
		}
	})

	t.Run("PauseExtendsPlayback", func(t *testing.T) {
		stream := NewBufferedStream(format, silentPCM(format, 300*time.Millisecond))
		go func() {
			time.Sleep(50 * time.Millisecond)
			SetStreamPaused(true)
			time.Sleep(150 * time.Millisecond)
			SetStreamPaused(false)
		}()

		started := time.Now()
		if err := PlayStream(context.Background(), stream); err != nil {
			t.Fatalf("PlayStream: %v", err)
		}
		if elapsed := time.Since(started); elapsed < 350*time.Millisecond {
			t.Fatalf("playback finished in %v despite a 150ms pause", elapsed)
		}
	})

	t.Run("UnsupportedFormatFails", func(t *testing.T) {
		stream := NewBufferedStream(StreamFormat{SampleRate: 24000, Channels: 1, BitDepth: 8}, []byte{1, 2})
		if err := PlayStream(context.Background(), stream); err == nil {
			t.Fatal("expected error for unsupported bit depth")
		}
	})
}
