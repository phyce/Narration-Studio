package tts

import (
	"fmt"
	"nstudio/app/tts/util"
	VoiceManger "nstudio/app/tts/voiceManager"
)

///////////////////////////////

func GenerateSpeech(messages []util.CharacterMessage, saveOutput bool) string {
	voiceManager := VoiceManger.GetInstance()
	//Get all required engines
	//Initialize and/or prepare each engine instance/model
	fmt.Println("In GenerateSpeech")
	//fmt.Println(messages)

	for _, message := range messages {
		fmt.Println("message character")
		fmt.Println(message.Character)
		fmt.Println("message save")
		fmt.Println(message.Save)
		voice := voiceManager.GetVoice(message.Character, message.Save)

		fmt.Println("name: ", voice.Name)
		fmt.Println("engine: ", voice.Engine)
		fmt.Println("model: ", voice.Model)
		fmt.Println("voice: ", voice.Voice)

		engine, ok := voiceManager.GetEngine(voice.Engine)
		if !ok {
			return "Error getting engine"
		}
		fmt.Println("\n\nengine: ")
		fmt.Println(engine)
		if saveOutput {
			fmt.Println("Saving Voice Clip")
			err := engine.Engine.Save([]util.CharacterMessage{message}, false)
			if err != nil {
				return "Error saving message: " + err.Error()
			}
		} else {
			err := engine.Engine.Play(message)
			if err != nil {
				return "Error playing message: " + err.Error()
			}
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
