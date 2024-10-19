package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"nstudio/app/common/eventManager"
	"nstudio/app/common/response"
	"nstudio/app/common/status"
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
	// TODO: combine with Play as they're mostly identical
	clearConsole()
	status.Set(status.Loading, "Processing Script")
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
	status.Set(status.Generating, "")
	err := tts.GenerateSpeech(messages, true)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to process script",
			Detail:  err.Error(),
		})
	} else {
		outputTypeRaw := []byte(config.GetInstance().GetSetting("outputType").Raw)

		if len(outputTypeRaw) == 0 {
			outputTypeRaw = []byte(*config.GetInstance().GetSetting("outputType").String)
		}

		fmt.Println("string(outputTypeRaw)")
		fmt.Println(string(outputTypeRaw))

		var outputType config.ConfigValueInt

		err = json.Unmarshal(outputTypeRaw, &outputType)
		if err != nil {
			response.Error(response.Data{
				Summary: "Failed to process config",
				Detail:  err.Error(),
			})
		}

		if outputType.Value == 0 || outputType.Value == 2 {
			//combined file OR both

			now := time.Now().Format("2006-01-02")
			dateString := now

			err, expandedPath := util.ExpandPath(*config.GetInstance().GetSetting("scriptOutputPath").String)
			if err != nil {
				response.Error(response.Data{
					Summary: "Failed to expand path",
					Detail:  err.Error(),
				})
			}

			outputPath := filepath.Join(
				expandedPath,
				dateString,
				util.FileTimestampGet(),
			)

			err = CombineWavFilesWithPause(
				outputPath,
				"combined.wav",
				time.Second,
				22050,
				1,
				16,
			)
			if err != nil {
				response.Error(response.Data{
					Summary: "Failed to combine wav files",
					Detail:  err.Error(),
				})
			}

			if outputType.Value != 2 {
				files, err := os.ReadDir(outputPath)
				if err != nil {
					response.Error(response.Data{
						Summary: "Failed to read directory",
						Detail:  err.Error(),
					})
					return // Make sure to return after handling the error
				}

				for _, file := range files {
					if !file.IsDir() && file.Name() != "combined.wav" {
						err = os.Remove(filepath.Join(outputPath, file.Name()))
						if err != nil {
							response.Error(response.Data{
								Summary: "Failed to delete file",
								Detail:  err.Error(),
							})
							return // Make sure to return after handling the error
						}
					}
				}
			}
		}

		response.Success(response.Data{
			Summary: "Success",
			Detail:  "Script processed successfully",
		})
		status.Set(status.Ready, "")
	}
}

//</editor-fold>

// <editor-fold desc="Character Voices">
func (app *App) GetCharacterVoices() string {
	status.Set(status.Loading, "Getting character voices")
	voices := voiceManager.GetInstance().CharacterVoices

	voicesJSON, err := json.Marshal(voices)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to get character voices",
			Detail:  err.Error(),
		})
	}

	fmt.Println("string(voicesJSON)")
	fmt.Println(string(voicesJSON))

	status.Set(status.Ready, "")
	return string(voicesJSON)
}

func (app *App) SaveCharacterVoices(voices string) {
	status.Set(status.Loading, "Saving character voices")
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
	status.Set(status.Ready, "")
}

func (app *App) GetAvailableModels() string {
	status.Set(status.Loading, "Getting available models")
	models := voiceManager.GetInstance().GetAllModels()

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

	voiceManager.GetInstance().ReloadModels()

	response.Success(response.Data{
		Summary: "Success",
		Detail:  "Voice Packs reloaded successfully",
	})
	status.Set(status.Ready, "")
}

//</editor-fold>

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
	status.Set(status.Loading, "Saving settings")
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
	status.Set(status.Ready, "")
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
			Title:            "Select Directory",
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
				Summary: "Directory changed",
			})
		} else {
			directory = defaultDirectory
		}
	}
	status.Set(status.Ready, "")
	return directory
}

func (app *App) RefreshModels() {
	clearConsole()
	status.Set(status.Loading, "Refreshing models")
	voiceManager.GetInstance().RefreshModels()
	response.Success(response.Data{
		Summary: "Models refreshed",
	})
	status.Set(status.Ready, "")
}

//</editor-fold>

// <editor-fold desc="Common">
func (app *App) GetEngines() string {
	engines := voiceManager.GetInstance().GetEngines()

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

func (app *App) GetStatus() string {
	status := status.Get()

	jsonData, err := json.Marshal(status)
	if err != nil {
		panic(err)
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

func CombineWavFilesWithPause(dirPath, outputFilename string, pauseDuration time.Duration, sampleRate, numChannels, bitDepth int) error {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	var wavFiles []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".wav" {
			wavFiles = append(wavFiles, filepath.Join(dirPath, file.Name()))
		}
	}

	if len(wavFiles) == 0 {
		return util.TraceError(fmt.Errorf("no WAV files found in directory"))
	}

	sort.Strings(wavFiles)

	var combinedBuffer *audio.IntBuffer

	// Create silence buffer
	silenceSamples := int(float64(pauseDuration.Seconds()) * float64(sampleRate))
	silenceData := make([]int, silenceSamples*numChannels)
	silenceBuffer := &audio.IntBuffer{
		Data: silenceData,
		Format: &audio.Format{
			NumChannels: numChannels,
			SampleRate:  sampleRate,
		},
		SourceBitDepth: bitDepth,
	}

	for idx, wavPath := range wavFiles {
		file, err := os.Open(wavPath)
		if err != nil {
			return err
		}

		decoder := wav.NewDecoder(file)
		if !decoder.IsValidFile() {
			file.Close()
			return util.TraceError(fmt.Errorf("invalid WAV file: " + wavPath))
		}

		buf, err := decoder.FullPCMBuffer()
		if err != nil {
			file.Close()
			return err
		}
		file.Close()

		// Resample if necessary
		if buf.Format.SampleRate != sampleRate {
			buf, err = ResampleBuffer(buf, sampleRate)
			if err != nil {
				return err
			}
		}

		// Change number of channels if necessary
		if buf.Format.NumChannels != numChannels {
			buf, err = ChangeNumChannels(buf, numChannels)
			if err != nil {
				return err
			}
		}

		// Change bit depth if necessary
		if buf.SourceBitDepth != bitDepth {
			buf, err = ChangeBitDepth(buf, bitDepth)
			if err != nil {
				return err
			}
		}

		if idx == 0 {
			combinedBuffer = &audio.IntBuffer{
				Data:           []int{},
				Format:         buf.Format,
				SourceBitDepth: buf.SourceBitDepth,
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
		return err
	}
	defer outFile.Close()

	encoder := wav.NewEncoder(outFile, sampleRate, bitDepth, numChannels, 1)
	err = encoder.Write(combinedBuffer)
	if err != nil {
		return err
	}

	err = encoder.Close()
	if err != nil {
		return err
	}

	// Optionally remove original WAV files
	/*
		for _, wavPath := range wavFiles {
			err := os.Remove(wavPath)
			if err != nil {
				return err
			}
		}
	*/

	return nil
}

func ResampleBuffer(buf *audio.IntBuffer, targetSampleRate int) (*audio.IntBuffer, error) {
	srcSampleRate := buf.Format.SampleRate
	if srcSampleRate == targetSampleRate {
		return buf, nil
	}

	resampleRatio := float64(targetSampleRate) / float64(srcSampleRate)
	srcData := buf.Data
	srcLength := len(srcData)
	dstLength := int(float64(srcLength) * resampleRatio)
	dstData := make([]int, dstLength)

	for i := 0; i < dstLength; i++ {
		srcPos := float64(i) / resampleRatio
		srcIndex := int(srcPos)
		if srcIndex >= srcLength-1 {
			srcIndex = srcLength - 2
		}
		frac := srcPos - float64(srcIndex)
		sample := int(float64(srcData[srcIndex])*(1-frac) + float64(srcData[srcIndex+1])*frac)
		dstData[i] = sample
	}

	resampledBuf := &audio.IntBuffer{
		Data:           dstData,
		Format:         &audio.Format{SampleRate: targetSampleRate, NumChannels: buf.Format.NumChannels},
		SourceBitDepth: buf.SourceBitDepth,
	}
	return resampledBuf, nil
}

func ChangeNumChannels(buf *audio.IntBuffer, targetNumChannels int) (*audio.IntBuffer, error) {
	srcNumChannels := buf.Format.NumChannels
	if srcNumChannels == targetNumChannels {
		return buf, nil
	}
	srcData := buf.Data
	numSamples := len(srcData) / srcNumChannels
	dstData := make([]int, numSamples*targetNumChannels)

	if targetNumChannels == 1 && srcNumChannels > 1 {
		// Convert from multi-channel to mono by averaging channels
		for i := 0; i < numSamples; i++ {
			sum := 0
			for c := 0; c < srcNumChannels; c++ {
				sum += srcData[i*srcNumChannels+c]
			}
			avg := sum / srcNumChannels
			dstData[i] = avg
		}
	} else if targetNumChannels > 1 && srcNumChannels == 1 {
		// Convert from mono to multi-channel by duplicating channels
		for i := 0; i < numSamples; i++ {
			sample := srcData[i]
			for c := 0; c < targetNumChannels; c++ {
				dstData[i*targetNumChannels+c] = sample
			}
		}
	} else {

		return nil, util.TraceError(fmt.Errorf("unsupported channel conversion"))
	}

	convertedBuf := &audio.IntBuffer{
		Data:           dstData,
		Format:         &audio.Format{SampleRate: buf.Format.SampleRate, NumChannels: targetNumChannels},
		SourceBitDepth: buf.SourceBitDepth,
	}
	return convertedBuf, nil
}

func ChangeBitDepth(buf *audio.IntBuffer, targetBitDepth int) (*audio.IntBuffer, error) {
	srcBitDepth := buf.SourceBitDepth
	if srcBitDepth == targetBitDepth {
		return buf, nil
	}

	srcData := buf.Data
	dstData := make([]int, len(srcData))

	maxSrcValue := 1 << (srcBitDepth - 1)
	maxDstValue := 1 << (targetBitDepth - 1)

	for i, sample := range srcData {
		// Scale sample value to target bit depth
		scaledSample := sample * maxDstValue / maxSrcValue
		dstData[i] = scaledSample
	}

	convertedBuf := &audio.IntBuffer{
		Data:           dstData,
		Format:         buf.Format,
		SourceBitDepth: targetBitDepth,
	}
	return convertedBuf, nil
}
