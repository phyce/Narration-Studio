package main

import (
	_ "embed"
	"encoding/json"
	"nstudio/app/common/issue"
	"nstudio/app/config"
	"sync"
)

//go:embed wails.json
var wailsJSON []byte

var info config.Info
var once sync.Once

func Info() config.Info {
	once.Do(func() {
		if err := json.Unmarshal(wailsJSON, &info); err != nil {
			issue.Panic(err)
		}
	})

	return info
}
