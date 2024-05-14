package tts

import (
	"hash/fnv"
	"math/rand"
	"time"
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
	Voices []Voice `json:"voices"`
}

type Voice struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Gender int    `json:"gender"`
}

type VoiceMessage struct {
	Character string `json:"character"`
	Text      string `json:"text"`
}

var CharacterVoices = map[string]Voice{}

func (voice *Voice) Synthesize(message string) {
	//figure out which engine
	//figure out which model
	//call engine with model and voice ID
	//return audio data
}

func GenerateSpeech(messages []VoiceMessage, save bool) string {
	//loop through each message
	//get character voice
	//save character voice to avoid figuring out again into CharacterVoices
	for _, message := range messages {
		if _, ok := CharacterVoices[message.Character]; !ok {
			CharacterVoices[message.Character] = selectVoice(message.Character)
		}

	}
	return ""
}

func selectVoice(character string) Voice {
	voices := getAllVoices()
	seed := hashStringToUint64(character)
	rand.Seed(int64(seed))
	randomIndex := rand.Intn(len(voices))
	return voices[randomIndex]
}

func hashStringToUint64(text string) uint64 {
	hash := fnv.New64a()
	_, err := hash.Write([]byte(text))
	if err != nil {
		return uint64(time.Now().UnixNano())
	}
	return hash.Sum64()
}
