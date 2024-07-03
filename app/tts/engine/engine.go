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
	Save(messages []util.CharacterMessage, play bool) error
	Generate(jsonBytes []byte) ([]byte, error)
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
	ID     string `json:"id"`
	Name   string `json:"name"`
	Engine string `json:"engine"`
}

func (m *Model) MarshalJSON() ([]byte, error) {
	// Define a temporary structure that includes all the fields you want to serialize
	type Alias Model
	return json.Marshal(&struct {
		*Alias
		Key string `json:"key"`
	}{
		Alias: (*Alias)(m),
		Key:   fmt.Sprintf("%s:%s", m.Engine, m.Name),
	})
}

type Voice struct {
	ID     string `json:"voiceID"`
	Name   string `json:"name"`
	Gender string `json:"gender"`
}

func (v *Voice) UnmarshalJSON(data []byte) error {
	// Define a helper struct with ID as an int
	type tempVoice struct {
		VoiceID      int    `json:"voiceID"`
		PiperVoiceID int    `json:"piperVoiceID"`
		Name         string `json:"name"`
		Gender       string `json:"gender"`
	}

	var tempStruct tempVoice
	if err := json.Unmarshal(data, &tempStruct); err != nil {
		return err
	}

	// Convert the int ID to a string and assign values
	v.ID = fmt.Sprintf("%d", tempStruct.VoiceID+tempStruct.PiperVoiceID)
	v.Name = tempStruct.Name
	v.Gender = tempStruct.Gender

	return nil
}
