package main

import (
	"encoding/json"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"nstudio/app/common/audio"
	"nstudio/app/common/eventManager"
	"nstudio/app/common/issue"
	"nstudio/app/common/response"
	"nstudio/app/common/status"
	"nstudio/app/common/util"
	"nstudio/app/common/util/fileIndex"
	"nstudio/app/config"
	"nstudio/app/enums/OutputType"
	"nstudio/app/tts"
	"nstudio/app/tts/voiceManager"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

//This is the entry point for all actions coming from the frontend

// <editor-fold desc="Sandbox">
/* TODO: combine with ProcessScript as they're mostly identical */
func (app *App) Play(
	script string,
	saveNewCharacters bool,
	overrideVoices string,
) {
	clearConsole()
	status.Set(status.Loading, "Playing")
	lines := strings.Split(script, "\n")
	var messages []util.CharacterMessage

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
			messages = append(messages, util.CharacterMessage{
				Character: character,
				Text:      text,
				Save:      saveNewCharacters,
			})
			response.Debug(response.Data{
				Summary: "added message by character: " + character,
				Detail:  text,
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

	status.Set(status.Ready, "")
}

//</editor-fold>

// <editor-fold desc="Script Editor">

func (app *App) ProcessScript(script string) {
	clearConsole()
	status.Set(status.Loading, "Processing Script")
	defer status.Set(status.Ready, "")

	lines := strings.Split(script, "\n")
	var messages []util.CharacterMessage

	regex := regexp.MustCompile(`^([^:]+):\s*(.*)$`)
	for _, line := range lines {
		if ttsLine := regex.FindStringSubmatch(line); ttsLine != nil {
			character := strings.TrimSpace(ttsLine[1])
			text := strings.TrimSpace(ttsLine[2])

			messages = append(messages, util.CharacterMessage{
				Character: character,
				Text:      text,
				Save:      true,
			})
		}
	}

	response.Debug(response.Data{
		Summary: "About to generate speech",
	})

	status.Set(status.Generating, "")
	err := tts.GenerateSpeech(messages, true)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to process script",
			Detail:  err.Error(),
		})
		return
	}

	outputType := config.GetSettings().OutputType

	if util.InArray(outputType, []OutputType.Option{OutputType.CombinedFile, OutputType.Both}) {
		now := time.Now().Format("2006-01-02")
		dateString := now

		err, expandedPath := util.ExpandPath(config.GetSettings().OutputPath)
		if err != nil {
			response.Error(response.Data{
				Summary: "Failed to expand path",
				Detail:  err.Error(),
			})
			return
		}

		outputPath := filepath.Join(
			expandedPath,
			dateString,
			fileIndex.Timestamp(),
		)

		err = audio.CombineWAVFiles(
			outputPath,
			"combined.wav",
			time.Second,
			48000,
			1,
			16,
		)
		if err != nil {
			response.Error(response.Data{
				Summary: "Failed to combine wav files",
				Detail:  err.Error(),
			})
			return
		}

		if outputType == OutputType.CombinedFile {
			files, err := os.ReadDir(outputPath)
			if err != nil {
				response.Error(response.Data{
					Summary: "Failed to read directory",
					Detail:  err.Error(),
				})
				return
			}

			for _, file := range files {
				if !file.IsDir() && file.Name() != "combined.wav" {
					err = os.Remove(filepath.Join(outputPath, file.Name()))
					if err != nil {
						response.Error(response.Data{
							Summary: "Failed to delete file",
							Detail:  err.Error(),
						})
						return
					}
				}
			}
		}
	}

	response.Success(response.Data{
		Summary: "Success",
		Detail:  "Script processed successfully",
	})
}

//</editor-fold>

// <editor-fold desc="Character Voices">
func (app *App) GetCharacterVoices() string {
	status.Set(status.Loading, "Getting character voices")
	voices := voiceManager.GetCharacterVoices()

	voicesJSON, err := json.Marshal(voices)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to get character voices",
			Detail:  err.Error(),
		})
	}

	status.Set(status.Ready, "")
	return string(voicesJSON)
}

func (app *App) SaveCharacterVoices(voices string) {
	status.Set(status.Loading, "Saving character voices")
	err := voiceManager.SaveCharacterVoices(voices)
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
	status.Set(status.Ready, "")
}

func (app *App) GetAvailableModels() string {
	status.Set(status.Loading, "Getting available models")
	models := voiceManager.GetAllModels()

	modelsJSON, err := json.Marshal(models)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to get available models",
			Detail:  err.Error(),
		})
	}
	status.Set(status.Ready, "")
	return string(modelsJSON)
}

//</editor-fold>

// <editor-fold desc="Voice Packs">
func (app *App) ReloadVoicePacks() {
	status.Set(status.Loading, "Reloading Voice Packs")

	voiceManager.ReloadModels()

	response.Success(response.Data{
		Summary: "Success",
		Detail:  "Voice Packs reloaded successfully",
	})
	status.Set(status.Ready, "")
}

//</editor-fold>

// <editor-fold desc="Settings">
func (app *App) GetSettings() config.Base {
	return config.Get()
}

func (app *App) GetSetting(name string) interface{} {
	value, err := config.GetValueFromPath(name)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to get setting",
			Detail:  err.Error(),
		})
		return ""
	}

	data, err := json.Marshal(value)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to marshal setting",
			Detail:  err.Error(),
		})
		return ""
	}

	return string(data)
}

func (app *App) SaveSettings(settings config.Base) {
	status.Set(status.Loading, "Saving settings")

	err := config.Set(settings)
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
	status.Set(status.Ready, "")
}

func (app *App) SelectDirectory(defaultDirectory string) string {
	status.Set(status.Loading, "Selecting directory")
	err, fullPath := util.ExpandPath(defaultDirectory)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to expand provided directory",
		})

		fullPath, err = os.UserHomeDir()
		if err != nil {
			response.Error(response.Data{
				Summary: "Failed to retrieve user's home directory.",
			})
			return ""
		}
	}

	directory, err := wailsRuntime.OpenDirectoryDialog(
		app.context,
		wailsRuntime.OpenDialogOptions{
			DefaultDirectory: fullPath,
			Title:            "Select Location",
		},
	)

	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to select directory",
			Detail:  err.Error(),
		})
	} else {
		if directory != "" {
			response.Success(response.Data{
				Summary: "Location changed",
			})
		} else {
			directory = defaultDirectory
		}
	}
	status.Set(status.Ready, "")
	return directory
}

func (app *App) SelectFile(defaultFile string) string {
	status.Set(status.Loading, "Selecting file")
	err, fullPath := util.ExpandPath(defaultFile)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to expand provided file path",
		})

		fullPath, err = os.UserHomeDir()
		if err != nil {
			response.Error(response.Data{
				Summary: "Failed to retrieve user's home directory.",
			})
			return ""
		}
	}

	file, err := wailsRuntime.OpenFileDialog(
		app.context,
		wailsRuntime.OpenDialogOptions{
			DefaultDirectory: filepath.Dir(fullPath),
			Title:            "Select File",
			Filters: []wailsRuntime.FileFilter{
				{
					DisplayName: "All Files",
					Pattern:     "*",
				},
			},
		},
	)

	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to select file",
			Detail:  err.Error(),
		})
	} else {
		if file != "" {
			response.Success(response.Data{
				Summary: "File selected",
			})
		} else {
			file = defaultFile
		}
	}
	status.Set(status.Ready, "")
	return file
}

func (app *App) RefreshModels() {
	clearConsole()
	status.Set(status.Loading, "Refreshing models")
	err := voiceManager.RefreshModels()
	if err == nil {
		response.Success(response.Data{
			Summary: "Models refreshed",
		})
		status.Set(status.Ready, "")
	} else {
		status.Set(status.Warning, "Some of your enabled engines didn't start")
	}

}

//</editor-fold>

// <editor-fold desc="Common">
func (app *App) GetEngines() string {
	engines := voiceManager.GetEngines()

	jsonData, err := json.Marshal(engines)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to get engines",
			Detail:  err.Error(),
		})
		return ""
	}

	return string(jsonData)
}

func (app *App) GetVoices(engine string, model string) string {
	voices, err := voiceManager.GetVoices(engine, model)
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

func (app *App) GetStatus() string {
	status := status.Get()

	jsonData, err := json.Marshal(status)
	if err != nil {
		issue.Panic("GetStatus failed: ", err)
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
