package tts

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sync"

	"nstudio/app/cache"
	"nstudio/app/common/audio"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/common/util/fileIndex"
)

type playbackSession struct {
	ctx    context.Context
	cancel context.CancelFunc
}

var (
	sessionMutex   sync.Mutex
	activeSession  *playbackSession
	sessionRunning sync.Mutex
)

func beginSession() (*playbackSession, func()) {
	sessionMutex.Lock()
	if activeSession != nil {
		activeSession.cancel()
	}
	sessionMutex.Unlock()

	sessionRunning.Lock()

	ctx, cancel := context.WithCancel(context.Background())
	session := &playbackSession{ctx: ctx, cancel: cancel}

	sessionMutex.Lock()
	activeSession = session
	sessionMutex.Unlock()

	SetPaused(false)

	endSession := func() {
		sessionMutex.Lock()
		if activeSession == session {
			activeSession = nil
		}
		sessionMutex.Unlock()
		cancel()
		SetPaused(false)
		sessionRunning.Unlock()
	}

	return session, endSession
}

func Stop() {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	if activeSession != nil {
		activeSession.cancel()
	}
}

var (
	pauseMutex   sync.Mutex
	resumeSignal = closedChannel()
)

func SetPaused(paused bool) {
	audio.SetStreamPaused(paused)

	pauseMutex.Lock()
	defer pauseMutex.Unlock()

	select {
	case <-resumeSignal:
		if paused {
			resumeSignal = make(chan struct{})
		}
	default:
		if !paused {
			close(resumeSignal)
		}
	}
}

func waitWhilePaused(ctx context.Context) {
	pauseMutex.Lock()
	currentResumeSignal := resumeSignal
	pauseMutex.Unlock()

	select {
	case <-currentResumeSignal:
	case <-ctx.Done():
	}
}

func closedChannel() chan struct{} {
	channel := make(chan struct{})
	close(channel)
	return channel
}

type audioFuture struct {
	message     util.CharacterMessage
	outputIndex int
	ready       chan struct{}
	stream      *audio.Stream
	err         error
	capture     *bytes.Buffer
	voiceKey    string
	fromCache   bool
}

func dispatchFutures(session *playbackSession, messages []util.CharacterMessage, saveOutput bool, profileID string) chan *audioFuture {
	queue := make(chan *audioFuture, 1)

	go func() {
		defer close(queue)
		for _, message := range messages {
			future := &audioFuture{
				message: message,
				ready:   make(chan struct{}),
			}
			if saveOutput {
				future.outputIndex = fileIndex.Get()
			}

			go produceFuture(session.ctx, future, profileID, saveOutput)

			select {
			case queue <- future:
			case <-session.ctx.Done():
				go discardFuture(future)
				return
			}
		}
	}()

	return queue
}

func produceFuture(ctx context.Context, future *audioFuture, profileID string, saveOutput bool) {
	defer close(future.ready)

	cacheManager := cache.GetManager()
	useCache := !saveOutput && cacheManager.IsEnabled()

	if useCache {
		if cached, found := cacheManager.GetCachedAudio(profileID, future.message.Character, future.message.Text); found {
			stream, err := streamFromCachedAudio(cached)
			if err == nil {
				future.stream = stream
				future.fromCache = true
				return
			}
			response.Err("ignoring unusable cache entry for %s: %w", future.message.Character, err)
		}
	}

	voice := future.message.Voice
	stream, err := SynthesizeStream(ctx, &voice, future.message.Text)
	if err != nil {
		future.err = err
		return
	}

	if useCache {
		future.capture = &bytes.Buffer{}
		future.voiceKey = fmt.Sprintf("%s:%s:%s", voice.Engine, voice.Model, voice.Voice)
		stream = audio.NewTeeStream(stream, future.capture)
	}

	future.stream = stream
}

func streamFromCachedAudio(cached []byte) (*audio.Stream, error) {
	if len(cached) == 0 {
		return nil, response.Err("cached audio is empty")
	}

	if len(cached) >= 12 && string(cached[0:4]) == "RIFF" && string(cached[8:12]) == "WAVE" {
		wavAudio, err := audio.NewAudioFromWAV(cached)
		if err != nil {
			return nil, err
		}
		return audio.NewStreamFromAudio(wavAudio)
	}

	return audio.NewBufferedStream(audio.StreamFormat{SampleRate: 22050, Channels: 1, BitDepth: 16}, cached), nil
}

func discardFuture(future *audioFuture) {
	<-future.ready
	if future.stream != nil {
		future.stream.Close()
	}
}

func drainFutures(queue chan *audioFuture) {
	for future := range queue {
		go discardFuture(future)
	}
}

func storeCacheCapture(future *audioFuture, profileID string) {
	if future.capture == nil || future.fromCache {
		return
	}
	if future.stream.Err() != nil || future.stream.TeeErr() != nil {
		return
	}

	capturedPCM := future.capture.Bytes()
	if len(capturedPCM) == 0 {
		return
	}

	format := future.stream.Format
	message := future.message
	voiceKey := future.voiceKey
	go func() {
		wavBytes, err := audio.NewAudioFromPCM(capturedPCM, format.SampleRate, format.Channels, format.BitDepth).ToWAV()
		if err != nil {
			response.Warn("Background caching failed: %v", err)
			return
		}
		if err := cache.GetManager().CacheAudio(profileID, message.Character, message.Text, voiceKey, wavBytes); err != nil {
			response.Warn("Background caching failed: %v", err)
		}
	}()
}

func saveFutureToFile(future *audioFuture, outputPath string) error {
	audioObject, err := future.stream.ReadAll()
	if err != nil {
		return err
	}

	wavBytes, err := audioObject.ToWAV()
	if err != nil {
		return err
	}

	filename := util.GenerateFilename(future.message, future.outputIndex, outputPath)
	return os.WriteFile(filename, wavBytes, 0644)
}
