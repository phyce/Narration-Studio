package main

import (
	"nstudio/app/config"
	"nstudio/app/tts"
	"regexp"
	"strings"
)

//This is the entry point for all actions coming from the frontend

//<editor-fold desc="Sandbox">
/* sandbox Play button action*/
func (app *App) Play(script string) string {
	lines := strings.Split(script, "\n")
	var messages []tts.VoiceMessage

	re := regexp.MustCompile(`^([^:]+):\s*(.*)$`)
	for _, line := range lines {
		if ttsLine := re.FindStringSubmatch(line); ttsLine != nil {
			character, text := ttsLine[1], ttsLine[2]
			messages = append(messages, tts.VoiceMessage{
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

//Character voices Start Preview button action
//Character voices Stop Preview button action (toggle?)

//save character voice settings button action
//preview character voice button action
//delete character voice button action

// toggle voice pack button action
