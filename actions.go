package main

import (
	"encoding/json"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"nstudio/app/common/eventManager"
	"nstudio/app/common/response"
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
/* TODO: combine with ProcessScript as they're mostly identical */
func (app *App) Play(script string, saveNewCharacters bool, overrideVoices string) {
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

	err := tts.GenerateSpeech(messages, false)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to play script",
			Detail:  err.Error(),
		})
	} else {
		response.Success(response.Data{
			Summary: "Success",
			Detail:  "Generation completed",
		})
	}
}

//</editor-fold>

// <editor-fold desc="Script Editor">

func (app *App) ProcessScript(script string) {
	// TODO: combine with Play as they're mostly identical
	clearConsole()
	lines := strings.Split(script, "\n")
	var messages []ttsUtil.CharacterMessage

	regex := regexp.MustCompile(`^([^:]+):\s*(.*)$`)
	for _, line := range lines {
		if ttsLine := regex.FindStringSubmatch(line); ttsLine != nil {
			character := strings.TrimSpace(ttsLine[1])
			text := strings.TrimSpace(ttsLine[2])

			messages = append(messages, ttsUtil.CharacterMessage{
				Character: character,
				Text:      text,
				Save:      true,
			})
		}
	}

	//TODO HERE
	//Need to sanitise input
	err := tts.GenerateSpeech(messages, true)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to process script",
			Detail:  err.Error(),
		})
	}
}

//</editor-fold>

// <editor-fold desc="Character Voices">
func (app *App) GetCharacterVoices() string {
	voices := voiceManager.GetInstance().CharacterVoices

	voicesJSON, err := json.Marshal(voices)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to get character voices",
			Detail:  err.Error(),
		})
	}

	return string(voicesJSON)
}

func (app *App) SaveCharacterVoices(voices string) {
	err := voiceManager.GetInstance().UpdateCharacterVoices(voices)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to save character voices",
			Detail:  err.Error(),
		})
	} else {
		response.Success(response.Data{
			Summary: "Successfully saved character voices",
		})
	}
}

func (app *App) GetAvailableModels() string {
	models := voiceManager.GetInstance().GetAllModels()

	modelsJSON, err := json.Marshal(models)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to get available models",
			Detail:  err.Error(),
		})
	}
	return string(modelsJSON)
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
		response.Error(response.Data{
			Summary: "Failed to get voices",
			Detail:  err.Error(),
		})
	}

	jsonData, err := json.Marshal(voices)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to get voices",
			Detail:  err.Error(),
		})
	}

	return string(jsonData)
}

//</editor-fold>

// <editor-fold desc="Events">
func (app *App) EventSubscribe(eventName string, handler func(data interface{})) {
	eventManager.GetInstance().SubscribeToEvent(eventName, handler)
}

func (a *App) EventTrigger(eventName string, data interface{}) {
	eventManager.GetInstance().TriggerEvent(eventName, data)
}

// </editor-fold>

// <editor-fold desc="Settings">
func (app *App) GetSettings() string {
	result, err := config.GetInstance().Export()
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to get settings",
			Detail:  err.Error(),
		})
	}

	return result
}

func (app *App) GetSetting(name string) string {
	result := config.GetInstance().GetSetting(name)
	data, err := json.Marshal(result)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to get setting",
			Detail:  err.Error(),
		})
	}
	return string(data)
}

func (app *App) SaveSettings(settings string) {
	err := config.GetInstance().Import(settings)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to save settings",
			Detail:  err.Error(),
		})
	} else {
		response.Success(response.Data{
			Summary: "Success",
			Detail:  "Settings have been saved",
		})
	}
}

func (app *App) SaveSetting(name string, newValue string) {
	var value config.Value

	err := json.Unmarshal([]byte(newValue), &value)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to unmarshal new value",
			Detail:  err.Error(),
		})
	}

	err = config.GetInstance().SetSetting(name, value)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to set new value",
			Detail:  err.Error(),
		})
	}
}

func (app *App) SelectDirectory(defaultDirectory string) string {
	directory, err := wailsRuntime.OpenDirectoryDialog(
		app.context,
		wailsRuntime.OpenDialogOptions{
			DefaultDirectory: defaultDirectory,
			Title:            "Select Directory",
		},
	)

	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to select directory",
			Detail:  err.Error(),
		})
	} else {
		response.Success(response.Data{
			Summary: "Directory changed",
		})
	}

	return directory
}

//</editor-fold>

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
