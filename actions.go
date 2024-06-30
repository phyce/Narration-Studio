package main

import (
	"encoding/json"
	"fmt"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
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

// <editor-fold desc="Sandbox">
// TODO: combine with ProcessScript as they're mostly identical
func (app *App) Play(script string, saveNewCharacters bool, overrideVoices string) string {
	clearConsole()
	lines := strings.Split(script, "\n")
	var messages []ttsUtil.CharacterMessage

	regex := regexp.MustCompile(`^([^:]+):\s*(.*)$`)
	for _, line := range lines {
		if ttsLine := regex.FindStringSubmatch(line); ttsLine != nil {
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

	return tts.GenerateSpeech(messages, false)
}

//</editor-fold>

// <editor-fold desc="Script Editor">

func (app *App) ProcessScript(script string) string {
	// TODO: combine with Play as they're mostly identical
	clearConsole()
	lines := strings.Split(script, "\n")
	var messages []ttsUtil.CharacterMessage

	regex := regexp.MustCompile(`^([^:]+):\s*(.*)$`)
	for _, line := range lines {
		if ttsLine := regex.FindStringSubmatch(line); ttsLine != nil {
			// Extract and sanitize the character and text
			character := strings.TrimSpace(ttsLine[1])
			text := strings.TrimSpace(ttsLine[2])

			fmt.Println("Character:", character)
			fmt.Println("Text:", text)

			// Append sanitized message to messages slice
			messages = append(messages, ttsUtil.CharacterMessage{
				Character: character,
				Text:      text,
				Save:      true,
			})
		}
	}

	//TODO HERE
	//Need to sanitise input
	return tts.GenerateSpeech(messages, true)
}

//</editor-fold>

// <editor-fold desc="Character Voices">
func (app *App) GetCharacterVoices() (string, error) {
	voices := voiceManager.GetInstance().CharacterVoices

	voicesJSON, err := json.Marshal(voices)
	if err != nil {
		return "", err
	}

	return string(voicesJSON), nil
}

func (app *App) SaveCharacterVoices(voices string) {
	voiceManager.GetInstance().UpdateCharacterVoices(voices)
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
	err := config.GetInstance().Import(settings)
	if err != nil {
		panic(err)
	}
}

func (app *App) SelectDirectory(defaultDirectory string) (string, error) {
	directory, err := wailsRuntime.OpenDirectoryDialog(
		app.ctx,
		wailsRuntime.OpenDialogOptions{
			DefaultDirectory: defaultDirectory,
			Title:            "Select Directory",
		},
	)

	if err != nil {
		return "", err
	}

	return directory, nil
}

//</editor-fold>

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
