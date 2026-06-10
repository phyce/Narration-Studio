//go:build !clib

package audio

import (
	"context"
	"encoding/binary"
	"nstudio/app/common/response"
	"sync"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/speaker"
)

const (
	playbackTargetRate        = 48000
	playbackPrebufferDuration = 200 * time.Millisecond
	playbackFeederChunkSize   = 8192
)

var playbackControl struct {
	mu     sync.Mutex
	ctrl   *beep.Ctrl
	paused bool
}

func SetStreamPaused(paused bool) {
	playbackControl.mu.Lock()
	defer playbackControl.mu.Unlock()
	playbackControl.paused = paused
	if playbackControl.ctrl != nil {
		speaker.Lock()
		playbackControl.ctrl.Paused = paused
		speaker.Unlock()
	}
}

func registerPlaybackControl(ctrl *beep.Ctrl) {
	playbackControl.mu.Lock()
	defer playbackControl.mu.Unlock()
	ctrl.Paused = playbackControl.paused
	playbackControl.ctrl = ctrl
}

func unregisterPlaybackControl(ctrl *beep.Ctrl) {
	playbackControl.mu.Lock()
	defer playbackControl.mu.Unlock()
	if playbackControl.ctrl == ctrl {
		playbackControl.ctrl = nil
	}
}

func PlayStream(ctx context.Context, stream *Stream) error {
	format := stream.Format
	if format.SampleRate <= 0 || format.BitDepth != 16 || (format.Channels != 1 && format.Channels != 2) {
		stream.Close()
		return response.Err("unsupported stream format for playback: %+v", format)
	}

	feeder := &playbackFeeder{channels: format.Channels}
	go feeder.consume(stream)

	bytesPerSecond := format.SampleRate * format.Channels * 2
	prebufferBytes := int(time.Duration(bytesPerSecond) * playbackPrebufferDuration / time.Second)
	for feeder.buffered() < prebufferBytes && !feeder.finishedReading() && !productionComplete(stream) {
		if ctx.Err() != nil {
			stream.Close()
			return ctx.Err()
		}
		time.Sleep(5 * time.Millisecond)
	}

	resampled := beep.Resample(4, beep.SampleRate(format.SampleRate), playbackTargetRate, beep.StreamerFunc(feeder.fill))
	ctrl := &beep.Ctrl{Streamer: resampled}
	registerPlaybackControl(ctrl)
	defer unregisterPlaybackControl(ctrl)

	finished := make(chan struct{})
	speaker.Play(beep.Seq(ctrl, beep.Callback(func() {
		close(finished)
	})))

	select {
	case <-finished:
		return stream.Err()
	case <-ctx.Done():
		speaker.Lock()
		ctrl.Streamer = nil
		ctrl.Paused = false
		speaker.Unlock()
		stream.Close()
		<-finished
		return ctx.Err()
	}
}

func productionComplete(stream *Stream) bool {
	select {
	case <-stream.Done():
		return true
	default:
		return false
	}
}

// playbackFeeder pulls the blocking stream reads off the beep callback path:
// consume() runs in its own goroutine, fill() runs inside the speaker mixer
// and must never block. On underrun it emits silence rather than stalling the
// device; beep's contract also forbids returning (0, true).
type playbackFeeder struct {
	mu       sync.Mutex
	queue    []byte
	finished bool
	channels int
}

func (feeder *playbackFeeder) consume(stream *Stream) {
	chunk := make([]byte, playbackFeederChunkSize)
	for {
		count, err := stream.Read(chunk)
		feeder.mu.Lock()
		if count > 0 {
			feeder.queue = append(feeder.queue, chunk[:count]...)
		}
		if err != nil {
			feeder.finished = true
			feeder.mu.Unlock()
			return
		}
		feeder.mu.Unlock()
	}
}

func (feeder *playbackFeeder) buffered() int {
	feeder.mu.Lock()
	defer feeder.mu.Unlock()
	return len(feeder.queue)
}

func (feeder *playbackFeeder) finishedReading() bool {
	feeder.mu.Lock()
	defer feeder.mu.Unlock()
	return feeder.finished
}

func (feeder *playbackFeeder) fill(samples [][2]float64) (int, bool) {
	feeder.mu.Lock()
	defer feeder.mu.Unlock()

	bytesPerFrame := 2 * feeder.channels
	frameCount := len(feeder.queue) / bytesPerFrame
	if frameCount > len(samples) {
		frameCount = len(samples)
	}

	for index := 0; index < frameCount; index++ {
		frame := feeder.queue[index*bytesPerFrame : (index+1)*bytesPerFrame]
		left := float64(int16(binary.LittleEndian.Uint16(frame[0:2]))) / (1 << 15)
		right := left
		if feeder.channels == 2 {
			right = float64(int16(binary.LittleEndian.Uint16(frame[2:4]))) / (1 << 15)
		}
		samples[index][0] = left
		samples[index][1] = right
	}
	feeder.queue = feeder.queue[frameCount*bytesPerFrame:]

	if frameCount == len(samples) {
		return frameCount, true
	}

	if feeder.finished {
		if frameCount == 0 {
			return 0, false
		}
		return frameCount, true
	}

	for index := frameCount; index < len(samples); index++ {
		samples[index] = [2]float64{}
	}
	return len(samples), true
}
