package tts

import (
	"fmt"
	"nstudio/app/common/status"
	"nstudio/app/common/util"
	"nstudio/app/tts/voiceManager"
)

func GenerateSpeech(messages []util.CharacterMessage, saveOutput bool) error {
	status.Set(status.Generating, "")

	util.FileIndexReset()
	for _, message := range messages {
		voice, err := voiceManager.GetVoice(message.Character, message.Save)
		if err != nil {
			return util.TraceError(err)
		}

		engine, ok := voiceManager.GetEngine(voice.Engine)
		if !ok {
			return util.TraceError(
				fmt.Errorf("Failed to retrieve engine: %s", voice.Engine),
			)
		}

		message.Voice = voice

		if saveOutput {
			err = engine.Engine.Save([]util.CharacterMessage{message}, false)
			if err != nil {
				return util.TraceError(err)
			}
		} else {
			status.Set(status.Playing, "")
			err = engine.Engine.Play(message)
			if err != nil {
				return util.TraceError(err)
			}
		}
	}
	voiceManager.ResetAllocatedVoices()
	return nil
}
