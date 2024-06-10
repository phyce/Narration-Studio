package engine

import (
	"encoding/json"
	"fmt"
	"nstudio/app/tts/util"
)

type Base interface {
	Initialize([]string) error
	Prepare() error
	Play(message util.CharacterMessage) error
	GetVoices(string) ([]Voice, error)
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
	ID     string `json:"voiceID"`
	Name   string `json:"name"`
	Gender string `json:"gender"`
}

func (v *Voice) UnmarshalJSON(data []byte) error {
	// Define a helper struct with ID as an int
	type helper struct {
		ID     int    `json:"voiceID"`
		Name   string `json:"name"`
		Gender string `json:"gender"`
	}

	var h helper
	if err := json.Unmarshal(data, &h); err != nil {
		return err
	}

	// Convert the int ID to a string and assign values
	v.ID = fmt.Sprintf("%d", h.ID)
	v.Name = h.Name
	v.Gender = h.Gender

	return nil
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
