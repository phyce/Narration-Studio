package voiceManager

import (
	"nstudio/app/common/util"
	tts "nstudio/app/tts/engine"
	"sync"
)

type VoiceManager struct {
	sync.Mutex
	Engines         map[string]tts.Engine
	CharacterVoices map[string]util.CharacterVoice
	AllocatedVoices map[string]util.CharacterVoice //These ones do not get saved permanently, only per run
}
