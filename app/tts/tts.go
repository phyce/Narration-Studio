package tts

import (
	"fmt"
	"nstudio/app/common/issue"
	"nstudio/app/common/status"
	"nstudio/app/common/util"
	"nstudio/app/common/util/fileIndex"
	"nstudio/app/tts/voiceManager"
)

func GenerateSpeech(messages []util.CharacterMessage, saveOutput bool) error {
	status.Set(status.Generating, "")

	fileIndex.Reset()
	for _, message := range messages {
		voice, err := voiceManager.GetVoice(message.Character, message.Save)
		if err != nil {
			return issue.Trace(err)
		}

		engine, ok := voiceManager.GetEngine(voice.Engine)
		if !ok {
			return issue.Trace(
				fmt.Errorf("Failed to retrieve engine: %s", voice.Engine),
			)
		}

		message.Voice = voice

		if saveOutput {
			err = engine.Engine.Save([]util.CharacterMessage{message}, false)
			if err != nil {
				return issue.Trace(err)
			}
		} else {
			status.Set(status.Playing, "")
			err = engine.Engine.Play(message)
			if err != nil {
				return issue.Trace(err)
			}
		}
	}
	voiceManager.ResetAllocatedVoices()
	return nil
}
