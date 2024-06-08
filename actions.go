package main

import (
	"encoding/json"
	"fmt"
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
	//if err := clearConsole(); err != nil {
	//	panic(err)
	//}
	clearConsole()
	fmt.Println("\n\n\n\n\n\n\n\n-----------------------------------")
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

//func (app *App) GetEngineVoiceData() string {
//	engines := voiceManager.GetInstance().GetEngines()
//
//	fmt.Println("In get engine voice data")
//	var voiceData []engine.Voice
//	for _, engineItem := range engines {
//		fmt.Println("engine")
//		fmt.Println(engineItem)
//		for _, model := range engine.Models {
//			fmt.Println("model")
//			fmt.Println(model)
//			for _, voice := range model.Voices {
//				fmt.Println("voice")
//				fmt.Println(voice)
//				voiceData = append(voiceData, voice)
//			}
//		}
//	}
//
//	engineString, err := json.Marshal(voiceData)
//	if err != nil {
//		fmt.Println(err)
//	}
//
//	return string(engineString)
//}

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
