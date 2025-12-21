package util

import (
	"encoding/json"
	"fmt"
)

type CharacterMessage struct {
	Character string         `json:"character"`
	Text      string         `json:"text"`
	Save      bool           `json:"save"`
	Voice     CharacterVoice `json:"-"`
}

type CharacterVoice struct {
	Name   string `json:"name"`
	Engine string `json:"engine"`
	Model  string `json:"model"`
	Voice  string `json:"voice"`
}

func (characterVoice *CharacterVoice) UnmarshalJSON(data []byte) error {
	type Alias CharacterVoice
	unmarshalTarget := &struct {
		*Alias
	}{
		Alias: (*Alias)(characterVoice),
	}
	if err := json.Unmarshal(data, &unmarshalTarget); err != nil {
		return err
	}
	return nil
}

func (characterVoice CharacterVoice) MarshalJSON() ([]byte, error) {
	type Alias CharacterVoice
	return json.Marshal(&struct {
		Key string `json:"key"`
		Alias
	}{
		Key:   fmt.Sprintf("%s:%s", characterVoice.Engine, characterVoice.Model),
		Alias: (Alias)(characterVoice),
	})
}

func (characterVoice *CharacterVoice) Key() string {
	return fmt.Sprintf("%s:%s:%s", characterVoice.Engine, characterVoice.Model, characterVoice.Voice)
}

type MessageData struct {
	Summary  string `json:"summary"`
	Detail   string `json:"detail"`
	Severity string `json:"severity"`
	Life     uint   `json:"life"`
}
