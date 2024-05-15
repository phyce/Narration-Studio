package tts

import (
	"nstudio/app/tts/util"
	VoiceManger "nstudio/app/tts/voiceManager"
)

type Engine struct {
	ID     int     `json:"id"`
	Name   string  `json:"name"`
	Models []Model `json:"models"`
}

type Model struct {
	ID     int     `json:"id"`
	Name   string  `json:"name"`
	Engine *string `json:"engine"`
	Voices []Voice `json:"voiceManager"`
}

type Voice struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Gender int    `json:"gender"`
}

///////////////////////////////

//func (voice *Voice) Synthesize(message string) {
//	//figure out which engine
//	//figure out which model
//	//call engine with model and voice ID
//	//return audio data
//}

func GenerateSpeech(messages []util.CharacterMessage, save bool) string {
	voiceManager := VoiceManger.GetInstance()
	for _, message := range messages {
		voice := voiceManager.GetVoice(message.Character)

		engine, ok := voiceManager.GetEngine(voice.Engine)
		if !ok {
			return "Error getting engine"
		}

		engine.Play(message)
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
