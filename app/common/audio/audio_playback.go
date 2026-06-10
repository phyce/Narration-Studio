//go:build !clib

package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"nstudio/app/common/response"
	"os"
	"sync"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/gopxl/beep"
	beepFlac "github.com/gopxl/beep/flac"
	beepMp3 "github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/mewkiz/flac"
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

func PlayPCMAudioBytes(audioClip []byte) error {
	audioDataReader := bytes.NewReader(audioClip)

	//TODO add ability to change format details
	originalFormat := beep.Format{
		SampleRate:  24000,
		NumChannels: 1,
		Precision:   2,
	}

	streamer := beep.StreamerFunc(func(samples [][2]float64) (processedSamples int, ok bool) {
		for sampleIndex := range samples {
			if audioDataReader.Len() < 2 {
				return sampleIndex, false
			}

			var sample int16
			err := binary.Read(audioDataReader, binary.LittleEndian, &sample)
			if err != nil {
				response.Err("Error reading PCM data: %w", err)
				return sampleIndex, false
			}

			floatSample := float64(sample) / (1 << 15)

			samples[sampleIndex][0] = floatSample
			samples[sampleIndex][1] = floatSample

			processedSamples++
		}
		return len(samples), true
	})

	resampler := beep.Resample(4, originalFormat.SampleRate, beep.SampleRate(48000), streamer)

	done := make(chan bool)

	speaker.Play(beep.Seq(resampler, beep.Callback(func() {
		done <- true
	})))

	<-done

	return nil
}

func PlayRawAudioBytes(audioClip []byte) {
	PlayRawAudioBytesAt(audioClip, 22050)
}

func PlayRawAudioBytesAt(audioClip []byte, sourceSampleRate int) {
	done := make(chan struct{})
	audioDataReader := bytes.NewReader(audioClip)

	streamer := beep.StreamerFunc(func(samples [][2]float64) (filledCount int, ok bool) {
		for index := range samples {
			if audioDataReader.Len() < 2 {
				return index, false
			}
			var sample int16

			err := binary.Read(audioDataReader, binary.LittleEndian, &sample)
			if err != nil {
				return index, false
			}
			floatSample := float64(sample) / (1 << 15)
			samples[index][0] = floatSample
			samples[index][1] = floatSample
		}
		return len(samples), true
	})

	resampledStreamer := beep.Resample(4, beep.SampleRate(sourceSampleRate), 48000, streamer)

	speaker.Play(beep.Seq(resampledStreamer, beep.Callback(func() {
		close(done)
	})))

	<-done
}

func PlayWAVBytes(wavData []byte) error {
	clip, err := NewAudioFromWAV(wavData)
	if err != nil {
		return response.Err("%w", err)
	}
	pcm, err := clip.ToPCM()
	if err != nil {
		return response.Err("%w", err)
	}
	PlayRawAudioBytesAt(pcm, clip.Metadata.SampleRate)
	return nil
}

func PlayFLACAudioBytes(audioClip []byte) error {
	audioReader := io.NopCloser(bytes.NewReader(audioClip))

	streamer, format, err := beepFlac.Decode(audioReader)
	if err != nil {
		return response.Err("%w", err)
	}
	defer streamer.Close()

	sampleRate := beep.SampleRate(48000)

	resampled := beep.Resample(4, format.SampleRate, sampleRate, streamer)

	done := make(chan bool)
	speaker.Play(beep.Seq(resampled, beep.Callback(func() {
		done <- true
	})))

	<-done

	return nil
}

func PlayMP3AudioBytes(audioClip []byte) error {
	audioReader := io.NopCloser(bytes.NewReader(audioClip))

	streamer, format, err := beepMp3.Decode(audioReader)
	if err != nil {
		return response.Err("%w", err)
	}
	defer streamer.Close()

	sampleRate := beep.SampleRate(48000)

	resampled := beep.Resample(4, format.SampleRate, sampleRate, streamer)

	done := make(chan bool)
	speaker.Play(beep.Seq(resampled, beep.Callback(func() {
		done <- true
	})))

	<-done

	return nil
}

func SaveFLACAsWAV(flacAudioClip []byte, filename string) error {
	reader := bytes.NewReader(flacAudioClip)

	stream, err := flac.New(reader)
	if err != nil {
		return response.Err("%w", err)
	}

	var buffer audio.IntBuffer
	buffer.Format = &audio.Format{
		NumChannels: int(stream.Info.NChannels),
		SampleRate:  int(stream.Info.SampleRate),
	}

	for {
		frame, err := stream.ParseNext()
		if err == io.EOF {
			break
		}
		if err != nil {
			return response.Err("%w", err)
		}
		for _, subframe := range frame.Subframes {
			for _, sample := range subframe.Samples {
				buffer.Data = append(buffer.Data, int(sample))
			}
		}
	}

	outputFile, err := os.Create(filename)
	if err != nil {
		return response.Err("%w", err)
	}
	defer outputFile.Close()

	encoder := wav.NewEncoder(outputFile, buffer.Format.SampleRate, int(stream.Info.BitsPerSample), buffer.Format.NumChannels, 1)
	if err := encoder.Write(&buffer); err != nil {
		return response.Err("%w", err)
	}
	if err := encoder.Close(); err != nil {
		return response.Err("%w", err)
	}

	return nil
}
