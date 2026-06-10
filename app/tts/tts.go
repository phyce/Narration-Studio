package tts

import (
	"context"

	"nstudio/app/common/audio"
	"nstudio/app/common/response"
	"nstudio/app/common/status"
	"nstudio/app/common/util"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/modelManager"
	"nstudio/app/tts/profile"
)

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
		select {
		case <-future.ready:
		case <-session.ctx.Done():
			go discardFuture(future)
			drainFutures(queue)
			return nil
		}

		if future.err != nil {
			session.cancel()
			drainFutures(queue)
			return response.Err("%w", future.err)
		}

		if saveOutput {
			if err := saveFutureToFile(future, outputPath); err != nil {
				session.cancel()
				drainFutures(queue)
				return response.Err("%w", err)
			}
			continue
		}

		status.Set(status.Playing, "")
		if err := audio.PlayStream(session.ctx, future.stream); err != nil {
			if session.ctx.Err() != nil {
				drainFutures(queue)
				return nil
			}
			session.cancel()
			drainFutures(queue)
			return response.Err("%w", err)
		}

		storeCacheCapture(future, profileID)
	}

	return nil
}

func SynthesizeStream(ctx context.Context, voice *util.CharacterVoice, text string) (*audio.Stream, error) {
	engineInstance, releaseInstance, ok := modelManager.GetEngineInstance(voice.Engine, voice.Model)
	if !ok {
		return nil, response.Err("Failed to retrieve engine instance: %s/%s", voice.Engine, voice.Model)
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
