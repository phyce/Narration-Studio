package config

import (
	"encoding/json"
	"sync"
)

// A Value can be either an int or a string
type Value struct {
	String *string `json:"string"`
	Int    *int    `json:"int"`
}

func (cv Value) MarshalJSON() ([]byte, error) {
	if cv.String != nil {
		return json.Marshal(cv.String)
	}
	return json.Marshal(cv.Int)
}

func (cv *Value) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil // it was null
	}
	if data[0] == '"' {
		return json.Unmarshal(data, &cv.String)
	}
	return json.Unmarshal(data, &cv.Int)
}

type ConfigManager struct {
	settings map[string]Value `json:"settings"`
	//voiceManager   map[string]Value
	filePath string     `json:"filePath"`
	lock     sync.Mutex `json:"lock"`
}
