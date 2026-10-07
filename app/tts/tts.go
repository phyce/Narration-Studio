package tts

import (
	"context"
	"errors"

	"nstudio/app/common/audio"
	"nstudio/app/common/response"
	"nstudio/app/common/status"
	"nstudio/app/common/util"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/modelManager"
	"nstudio/app/tts/profile"
)

var ErrStopped = errors.New("speech generation stopped")

func GenerateSpeech(messages []util.CharacterMessage, saveOutput bool, profileID string) error {
	session, endSession := beginSession()
	defer endSession()

	status.Set(status.Generating, "Generating Audio")
	defer status.Set(status.Ready, "")

	profileManager := profile.GetManager()
	resolved := make([]util.CharacterMessage, 0, len(messages))
	for _, message := range messages {
		voice, err := profileManager.GetOrAllocateVoice(profileID, message.Character)
		if err != nil {
			return response.Err("%w", err)
		}
		message.Voice = *voice
		resolved = append(resolved, message)
	}

	outputPath := ""
	if saveOutput {
		err, expandedPath := util.ExpandPath(config.GetSettings().OutputPath)
		if err != nil {
			return response.Err("%w", err)
		}
		outputPath = expandedPath
	}

	queue := dispatchFutures(session, resolved, saveOutput, profileID)

	for future := range queue {
		if saveOutput {
			waitWhilePaused(session.ctx)
		}

		select {
		case <-future.ready:
		case <-session.ctx.Done():
		}

		if session.ctx.Err() != nil {
			go discardFuture(future)
			drainFutures(queue)
			return ErrStopped
		}

		if future.err != nil {
			return abortSession(session, queue, future.err)
		}

		if saveOutput {
			if err := saveFutureToFile(future, outputPath); err != nil {
				return abortSession(session, queue, err)
			}
			continue
		}

		status.Set(status.Playing, "")
		if err := audio.PlayStream(session.ctx, future.stream); err != nil {
			return abortSession(session, queue, err)
		}

		storeCacheCapture(future, profileID)
	}

	return nil
}

func abortSession(session *playbackSession, queue chan *audioFuture, err error) error {
	stopped := session.ctx.Err() != nil
	session.cancel()
	drainFutures(queue)

	if stopped {
		return ErrStopped
	}
	return response.Err("%w", err)
}

func SynthesizeStream(ctx context.Context, voice *util.CharacterVoice, text string) (*audio.Stream, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	engineInstance, releaseInstance, ok := modelManager.GetEngineInstance(voice.Engine, voice.Model)
	if !ok {
		if modelManager.IsReleased() {
			return nil, response.Err("%s/%s is hosted by the background server; stop the server to use it here", voice.Engine, voice.Model)
		}
		return nil, response.Err("Failed to retrieve engine instance: %s/%s", voice.Engine, voice.Model)
	}

	if err := ctx.Err(); err != nil {
		releaseInstance()
		return nil, err
	}

	stream, err := engineInstance.Synthesize(ctx, engine.SpeechRequest{
		Model: voice.Model,
		Voice: voice.Voice,
		Text:  text,
	})
	if err != nil {
		releaseInstance()
		return nil, response.Err("%w", err)
	}

	go func() {
		<-stream.Done()
		releaseInstance()
	}()

	return stream, nil
}

func GenerateAudio(voice *util.CharacterVoice, text string) (*audio.Audio, error) {
	stream, err := SynthesizeStream(context.Background(), voice, text)
	if err != nil {
		return nil, response.Err("%w", err)
	}

	return stream.ReadAll()
}

func GenerateRawAudio(voice *util.CharacterVoice, text string) ([]byte, error) {
	audioObject, err := GenerateAudio(voice, text)
	if err != nil {
		return nil, response.Err("%w", err)
	}

	return audioObject.ToPCM()
}
