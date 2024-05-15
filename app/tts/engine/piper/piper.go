package piper

import (
	"fmt"
	"nstudio/app/tts/util"
)

type Piper struct{}

func (piper *Piper) Initialize() {
	fmt.Println("Piper engine initialized")
}

func (piper *Piper) Play(message util.CharacterMessage) {
	fmt.Printf("Piper playing: VoiceID=%s, Message=%s\n", message.Character, message.Text)
}
