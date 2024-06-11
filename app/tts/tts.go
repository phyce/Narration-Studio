package tts

import (
	"nstudio/app/tts/util"
	VoiceManger "nstudio/app/tts/voiceManager"
)

///////////////////////////////

func GenerateSpeech(messages []util.CharacterMessage) string {
	voiceManager := VoiceManger.GetInstance()
	//Get all required engines
	//Initialize and/or prepare each engine instance/model
	for _, message := range messages {
		voice := voiceManager.GetVoice(message.Character, message.Save)

		engine, ok := voiceManager.GetEngine(voice.Engine)
		if !ok {
			return "Error getting engine"
		}
		err := engine.Engine.Play(message)
		if err != nil {
			return "Error playing message"
		}
	}

	//loop through each message
	//get character voice
	//save character voice to avoid figuring out again into CharacterVoices
	//for _, message := range messages {
	//	if _, ok := CharacterVoices[message.Character]; !ok {
	//		CharacterVoices[message.Character] = selectVoice(message.Character)
	//	}
	//
	//}
	return ""
}
