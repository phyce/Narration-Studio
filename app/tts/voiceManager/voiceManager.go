package voiceManager

import (
	"nstudio/app/tts/engine"
	"sync"
)

type CharacterVoice struct {
	Name   string `json:"name"`
	Engine string `json:"engine"`
	Model  string `json:"model"`
	Voice  int    `json:"voice"`
}

type VoiceManager struct {
	sync.Mutex
	Engines         map[string]engine.EngineBase
	CharacterVoices map[string]CharacterVoice
}

var (
	instance *VoiceManager
	once     sync.Once
)

func GetInstance() *VoiceManager {
	once.Do(func() {
		instance = &VoiceManager{
			Engines:         make(map[string]engine.EngineBase),
			CharacterVoices: make(map[string]CharacterVoice),
		}
	})
	return instance
}

func (manager *VoiceManager) GetVoice(name string) CharacterVoice {
	manager.Lock()
	defer manager.Unlock()

	if _, ok := manager.CharacterVoices[name]; !ok {
		manager.CharacterVoices[name] = CharacterVoice{
			Name:   name,
			Engine: "piper",    //TODO: Add logic to select engine
			Model:  "libritts", //TODO: Add logic to select model
			Voice:  0,          //TODO: Add logic to select voice
		}
	}

	return manager.CharacterVoices[name]
}

func (manager *VoiceManager) RegisterEngine(name string, engine engine.EngineBase) {
	manager.Lock()
	defer manager.Unlock()

	manager.Engines[name] = engine
}

func (manager *VoiceManager) GetEngine(name string) (engine.EngineBase, bool) {
	selectedEngine, ok := manager.Engines[name]
	return selectedEngine, ok
}

func (manager *VoiceManager) UnregisterEngine(name string) {
	manager.Lock()
	defer manager.Unlock()

	delete(manager.Engines, name)
}

//func selectVoice(character string) Voice {
//	voices := getAllVoices()
//	seed := hashStringToUint64(character)
//	rand.Seed(int64(seed))
//	randomIndex := rand.Intn(len(voices))
//	return voices[randomIndex]
//}
//
//func hashStringToUint64(text string) uint64 {
//	hash := fnv.New64a()
//	_, err := hash.Write([]byte(text))
//	if err != nil {
//		return uint64(time.Now().UnixNano())
//	}
//	return hash.Sum64()
//}
