package main

import (
	"fmt"
	"nstudio/app/config"
	"nstudio/app/tts"
	ttsUtil "nstudio/app/tts/util"
	"regexp"
	"strings"
)

//This is the entry point for all actions coming from the frontend

//<editor-fold desc="Sandbox">
/* sandbox Play button action*/
func (app *App) Play(script string) string {
	lines := strings.Split(script, "\n")
	var messages []ttsUtil.CharacterMessage

	re := regexp.MustCompile(`^([^:]+):\s*(.*)$`)
	for _, line := range lines {
		if ttsLine := re.FindStringSubmatch(line); ttsLine != nil {
			character, text := ttsLine[1], ttsLine[2]
			fmt.Println(character, text)
			messages = append(messages, ttsUtil.CharacterMessage{
				Character: character,
				Text:      text,
			})
		}
	}

	return tts.GenerateSpeech(messages, false)
}

//</editor-fold>

// <editor-fold desc="Settings">
func (app *App) GetSettings() string {
	result, error := config.GetInstance().Export()
	if error != nil {
		panic(error)
	}
	return result
}

func (app *App) SaveSettings(settings string) {
	error := config.GetInstance().Import(settings)
	if error != nil {
		panic(error)
	}
}

//</editor-fold>

//script editor Generate button action

//Character voiceManager Start Preview button action
//Character voiceManager Stop Preview button action (toggle?)

//save character voice settings button action
//preview character voice button action
//delete character voice button action

// toggle voice pack button action
