package engine

import (
	"encoding/json"
	"fmt"
	"nstudio/app/common/util"
)

type Base interface {
	Initialize() error
	Start(modelName string) error
	Stop(modelName string) error
	Play(message util.CharacterMessage) error
	Save(messages []util.CharacterMessage, play bool) error
	Generate(model string, payload []byte) ([]byte, error)
	GetVoices(model string) ([]Voice, error)
	FetchModels() map[string]Model
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

func (model *Model) MarshalJSON() ([]byte, error) {
	type Alias Model
	return json.Marshal(&struct {
		*Alias
		Key string `json:"key"`
	}{
		Alias: (*Alias)(model),
		Key:   fmt.Sprintf("%s:%s", model.Engine, model.Name),
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
