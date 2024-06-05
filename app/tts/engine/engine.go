package engine

import (
	"nstudio/app/tts/util"
)

type Base interface {
	Initialize() error
	Prepare() error
	Play(message util.CharacterMessage) error
	//GetModels() []Model
}

type Engine struct {
	Engine Base             `json:"-"`
	ID     string           `json:"id"`
	Name   string           `json:"name"`
	Models map[string]Model `json:"models"`
}

type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	//Voices []Voice `json:"voices"`
	//Engine *string `json:"engine"`
	//Voices []Voice `json:"voiceManager"`
}

type Voice struct {
	ID     int    `json:"piperVoiceID"`
	Name   string `json:"name"`
	Gender string `json:"gender"`
}

//func (engine *Engine) Initialize() error {
//	//tType := reflect.TypeOf((*T)(nil)).Elem()
//	//panic(fmt.Sprintf("Initialize for %s engine has not been defined", tType.Name()))
//	panic(fmt.Sprintf("Initialize for engine has not been defined"))
//}
//
//func (engine *Engine) Play(message util.CharacterMessage) error {
//	fmt.Println("Base play: %s", message.Text)
//	return nil
//}
//
//func (engine *Engine) Prepare() error {
//	fmt.Println("Base prepare: %s")
//	return nil
//}
//
//func (engine *Engine) GetID() string {
//	return "not set"
//}
//
//func (engine *Engine) GetName() string {
//	return "not set"
//}
