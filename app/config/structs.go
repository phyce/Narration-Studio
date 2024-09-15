package config

import (
	"encoding/json"
	"sync"
)

// A Value can be either an int or a string
type Value struct {
	String *string `json:"string"`
	Int    *int    `json:"int"`
	Raw    string  `json:"raw"`
}

func (cv Value) MarshalJSON() ([]byte, error) {
	if cv.String != nil {
		return json.Marshal(cv.String)
	}
	if cv.Raw != "" {
		return json.Marshal(cv.Raw)
	}
	return json.Marshal(cv.Int)
}

func (cv *Value) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	if data[0] == '"' {
		return json.Unmarshal(data, &cv.String)
	}
	if data[0] == '{' || data[0] == '[' {
		cv.Raw = string(data)
		return nil
	}
	return json.Unmarshal(data, &cv.Int)
}

type ConfigManager struct {
	settings map[string]Value `json:"settings"`
	//voiceManager   map[string]Value
	filePath string     `json:"filePath"`
	lock     sync.Mutex `json:"lock"`
}
