package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"nstudio/app/common/eventManager"
	"nstudio/app/common/response"
	"nstudio/app/config"
	"nstudio/app/tts"
	util "nstudio/app/tts/util"
	"nstudio/app/tts/voiceManager"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

//This is the entry point for all actions coming from the frontend

// <editor-fold desc="Sandbox">
/* TODO: combine with ProcessScript as they're mostly identical */
func (app *App) Play(script string, saveNewCharacters bool, overrideVoices string) {
	clearConsole()
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
	//TODO: Need to sanitise input
	err := tts.GenerateSpeech(messages, true)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to process script",
			Detail:  err.Error(),
		})
	} else {
		outputTypeRaw := []byte(config.GetInstance().GetSetting("outputType").Raw)
		var outputType config.ConfigValueInt

		err = json.Unmarshal(outputTypeRaw, &outputType)
		if err != nil {
			response.Error(response.Data{
				Summary: "Failed to process config",
				Detail:  err.Error(),
			})
		}

		if outputType.Value == 0 {
			//combined file

			now := time.Now()
			dateString := now.Format("2006-01-02")

			err, expandedPath := util.ExpandPath(*config.GetInstance().GetSetting("scriptOutputPath").String)
			if err != nil {
				response.Error(response.Data{
					Summary: "Failed to expand path",
					Detail:  err.Error(),
				})
			}

			outputPath := filepath.Join(expandedPath, dateString)

			timeString := now.Format("15-04-05.wav")

			err = CombineWavFilesWithPause(outputPath, timeString, time.Second, 22050)
			if err != nil {
				response.Error(response.Data{
					Summary: "Failed to combine wav files",
					Detail:  err.Error(),
				})
			}
		}

		response.Success(response.Data{
			Summary: "Success",
			Detail:  "Script processed successfully",
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
	fmt.Println(voices)
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

func (app *App) RefreshModels() {
	clearConsole()
	voiceManager.GetInstance().RefreshModels()
	response.Success(response.Data{
		Summary: "Models refreshed",
	})
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

func CombineWavFilesWithPause(dirPath, outputFilename string, pauseDuration time.Duration, sampleRate int) error {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return util.TraceError(err)
	}

	var wavFiles []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".wav" {
			wavFiles = append(wavFiles, filepath.Join(dirPath, file.Name()))
		}
	}

	if len(wavFiles) == 0 {
		return util.TraceError(errors.New("No WAV files found in directory"))
	}

	sort.Strings(wavFiles)

	var combinedBuffer *audio.IntBuffer
	var numChannels int
	var bitDepth int

	silenceSamples := int(float64(pauseDuration.Seconds()) * float64(sampleRate))
	silenceBuffer := &audio.IntBuffer{
		Data: make([]int, silenceSamples*2),
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  sampleRate,
		},
	}

	for idx, wavPath := range wavFiles {
		file, err := os.Open(wavPath)
		if err != nil {
			return util.TraceError(err)
		}

		decoder := wav.NewDecoder(file)
		if !decoder.IsValidFile() {
			file.Close()
			return util.TraceError(errors.New("Invalid WAV file:" + wavPath))
		}

		buf, err := decoder.FullPCMBuffer()
		if err != nil {
			file.Close()
			return util.TraceError(err)
		}

		file.Close()

		if idx == 0 {
			combinedBuffer = &audio.IntBuffer{
				Data:           []int{},
				Format:         buf.Format,
				SourceBitDepth: buf.SourceBitDepth,
			}
			numChannels = buf.Format.NumChannels
			bitDepth = buf.SourceBitDepth

			if buf.Format.SampleRate != sampleRate {
				return util.TraceError(errors.New("Sample rate mismatch:" + wavPath))
			}
		} else {
			if buf.Format.SampleRate != sampleRate ||
				buf.Format.NumChannels != numChannels ||
				buf.SourceBitDepth != bitDepth {
				return util.TraceError(errors.New("Audio format mismatch:" + wavPath))
			}
		}

		combinedBuffer.Data = append(combinedBuffer.Data, buf.Data...)

		if idx < len(wavFiles)-1 {
			combinedBuffer.Data = append(combinedBuffer.Data, silenceBuffer.Data...)
		}
	}

	outputPath := filepath.Join(dirPath, outputFilename)
	outFile, err := os.Create(outputPath)
	if err != nil {
		return util.TraceError(err)
	}
	defer outFile.Close()

	encoder := wav.NewEncoder(outFile, sampleRate, bitDepth, numChannels, 1)
	err = encoder.Write(combinedBuffer)
	if err != nil {
		return util.TraceError(err)
	}

	err = encoder.Close()
	if err != nil {
		return util.TraceError(err)
	}

	for _, wavPath := range wavFiles {
		err := os.Remove(wavPath)
		if err != nil {
			return util.TraceError(err)
		}
	}

	return nil
}
