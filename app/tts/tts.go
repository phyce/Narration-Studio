package tts

import (
	"fmt"
	"nstudio/app/tts/util"
	VoiceManger "nstudio/app/tts/voiceManager"
)

func GenerateSpeech(messages []util.CharacterMessage, saveOutput bool) error {
	voiceManager := VoiceManger.GetInstance()

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

		if saveOutput {
			err := engine.Engine.Save([]util.CharacterMessage{message}, false)
			if err != nil {
				return util.TraceError(err)
			}
		} else {
			err := engine.Engine.Play(message)
			if err != nil {
				return util.TraceError(err)
			}
		}
	}
	return nil
}
