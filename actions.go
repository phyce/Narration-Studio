package main

import (
	"encoding/json"
	"nstudio/app/config"
	"nstudio/app/tts"
	ttsUtil "nstudio/app/tts/util"
	"nstudio/app/tts/voiceManager"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

//This is the entry point for all actions coming from the frontend

//<editor-fold desc="Sandbox">
/* sandbox Play button action*/
func (app *App) Play(script string, saveNewCharacters bool, overrideVoices string) string {
	clearConsole()
	lines := strings.Split(script, "\n")
	var messages []ttsUtil.CharacterMessage

	re := regexp.MustCompile(`^([^:]+):\s*(.*)$`)
	for _, line := range lines {
		if ttsLine := re.FindStringSubmatch(line); ttsLine != nil {
			var character string
			if overrideVoices != "" {
				character = overrideVoices
			} else {
				character = ttsLine[1]
			}
			text := ttsLine[2]
			messages = append(messages, ttsUtil.CharacterMessage{
				Character: character,
				Text:      text,
				Save:      saveNewCharacters,
			})
		}
	}

	return tts.GenerateSpeech(messages)
}

//</editor-fold>

// <editor-fold desc="Common">
func (app *App) GetEngines() string {
	engines := voiceManager.GetInstance().GetEngines()

	jsonData, err := json.Marshal(engines)
	if err != nil {
		panic(err)
	}

	return string(jsonData)
}

func (app *App) GetVoices(engine string, model string) string {
	voices, err := voiceManager.GetInstance().GetVoices(engine, model)
	if err != nil {
		return "Failed to get voices: " + err.Error()
	}

	jsonData, err := json.Marshal(voices)
	if err != nil {
		return "Failed to marshal voices: " + err.Error()
	}

	return string(jsonData)
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

func clearConsole() error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls") // Clear console command for Windows
	} else {
		cmd = exec.Command("clear") // Clear console command for Linux and MacOS
	}
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
