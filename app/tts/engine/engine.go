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
	Generate(model string, jsonBytes []byte) ([]byte, error)
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
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Engine   string        `json:"engine"`
	Download ModelDownload `json:"modelDownload"`
}

type ModelDownload struct {
	Metadata string `json:"metadata"`
	Model    string `json:"model"`
	Phonemes string `json:"phonemes"`
}

func (m *Model) MarshalJSON() ([]byte, error) {
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

func (voice *Voice) UnmarshalJSON(data []byte) error {
	type tempVoice struct {
		VoiceID      int    `json:"voiceID"`
		PiperVoiceID int    `json:"piperVoiceID"`
		Name         string `json:"name"`
		Gender       string `json:"gender"`
	}

	var tempStruct tempVoice
	if err := json.Unmarshal(data, &tempStruct); err != nil {
		return util.TraceError(err)
	}

	voice.ID = fmt.Sprintf("%d", tempStruct.VoiceID+tempStruct.PiperVoiceID)
	voice.Name = tempStruct.Name
	voice.Gender = tempStruct.Gender

	return nil
}
