//go:build !cli && !clib

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"nstudio/app/cache"
	"nstudio/app/common/activity"
	"nstudio/app/common/audio"
	"nstudio/app/common/eventManager"
	"nstudio/app/common/issue"
	"nstudio/app/common/process"
	"nstudio/app/common/response"
	"nstudio/app/common/status"
	"nstudio/app/common/util"
	"nstudio/app/common/util/fileIndex"
	"nstudio/app/config"
	"nstudio/app/enums/OutputType"
	"nstudio/app/tts"
	"nstudio/app/tts/engine/piper"
	"nstudio/app/tts/engine/piper/native"
	"nstudio/app/tts/engine/python/pack"
	"nstudio/app/tts/enginehost"
	"nstudio/app/tts/modelManager"
	"nstudio/app/tts/profile"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// <editor-fold desc="Sandbox">
func (app *App) Play(
	script string,
	saveNewCharacters bool,
	overrideVoices string,
	profileID string,
) {
	clearConsole()
	status.Set(status.Loading, "Generating Audio To Play")

	if profileID == "" {
		profileID = "default"
	}

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
			response.Debug(util.MessageData{
				Summary: "added message by character: " + character,
				Detail:  text,
			})
		}
	}

	fileIndex.Reset()
	err := tts.GenerateSpeech(messages, false, profileID)
	switch {
	case errors.Is(err, tts.ErrStopped):
		response.Info(util.MessageData{Summary: "Playback stopped"})
	case err != nil:
		response.Error(util.MessageData{
			Summary: "Failed to play script",
			Detail:  err.Error(),
			Cause:   err,
		})
	default:
		response.Success(util.MessageData{
			Summary: "Success",
			Detail:  "Generation completed",
		})
	}

	status.Set(status.Ready, "")
}

func (app *App) StopPlayback() {
	tts.Stop()
}

func (app *App) SetPlaybackPaused(paused bool) {
	tts.SetPaused(paused)
}

//</editor-fold>

// <editor-fold desc="Script Editor">

func (app *App) ProcessScript(script string, profileID string) {
	clearConsole()
	status.Set(status.Loading, "Processing Script")
	defer status.Set(status.Ready, "")

	if profileID == "" {
		profileID = "default"
	}

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

	response.Debug(util.MessageData{
		Summary: "About to generate speech",
	})

	status.Set(status.Generating, "")
	fileIndex.Reset()
	err := tts.GenerateSpeech(messages, true, profileID)
	if errors.Is(err, tts.ErrStopped) {
		response.Info(util.MessageData{Summary: "Script processing stopped"})
		return
	}
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to process script",
			Detail:  err.Error(),
			Cause:   err,
		})
		return
	}

	outputType := config.GetSettings().OutputType

	if util.InArray(outputType, []OutputType.Option{OutputType.CombinedFile, OutputType.Both}) {
		now := time.Now().Format("2006-01-02")
		dateString := now

		err, expandedPath := util.ExpandPath(config.GetSettings().OutputPath)
		if err != nil {
			response.Error(util.MessageData{
				Summary: "Failed to expand path",
				Detail:  err.Error(),
				Cause:   err,
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
			response.Error(util.MessageData{
				Summary: "Failed to combine wav files",
				Detail:  err.Error(),
				Cause:   err,
			})
			return
		}

		if outputType == OutputType.CombinedFile {
			files, err := os.ReadDir(outputPath)
			if err != nil {
				response.Error(util.MessageData{
					Summary: "Failed to read directory",
					Detail:  err.Error(),
					Cause:   err,
				})
				return
			}

			for _, file := range files {
				if !file.IsDir() && file.Name() != "combined.wav" {
					err = os.Remove(filepath.Join(outputPath, file.Name()))
					if err != nil {
						response.Error(util.MessageData{
							Summary: "Failed to delete file",
							Detail:  err.Error(),
							Cause:   err,
						})
						return
					}
				}
			}
		}
	}

	response.Success(util.MessageData{
		Summary: "Success",
		Detail:  "Script processed successfully",
	})
}

//</editor-fold>

// <editor-fold desc="Profiles">

func (app *App) GetAvailableModels() string {
	status.Set(status.Loading, "Getting available models")
	models := modelManager.GetAllModels()

	modelsJSON, err := json.Marshal(models)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to get available models",
			Detail:  err.Error(),
			Cause:   err,
		})
	}
	status.Set(status.Ready, "")
	return string(modelsJSON)
}

//</editor-fold>

// <editor-fold desc="Voice Packs">
func (app *App) ReloadVoicePacks() {
	status.Set(status.Loading, "Reloading Voice Packs")

	modelManager.ReloadModels()

	response.Success(util.MessageData{
		Summary: "Success",
		Detail:  "Voice Packs reloaded successfully",
	})
	status.Set(status.Ready, "")
}

func (app *App) CheckApiEngines() string {
	availability := modelManager.CheckApiEngines(context.Background())
	availabilityJSON, err := json.Marshal(availability)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to check the online engines",
			Detail:  err.Error(),
			Cause:   err,
		})
		return "{}"
	}
	return string(availabilityJSON)
}

//</editor-fold>

// <editor-fold desc="Settings">
func (app *App) GetSettings() config.Base {
	return config.Get()
}

func (app *App) GetSetting(name string) interface{} {
	value, err := config.GetValueFromPath(name)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to get setting",
			Detail:  err.Error(),
			Cause:   err,
		})
		return ""
	}

	data, err := json.Marshal(value)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to marshal setting",
			Detail:  err.Error(),
			Cause:   err,
		})
		return ""
	}

	return string(data)
}

func (app *App) SaveSettings(settings config.Base) {
	status.Set(status.Loading, "Saving settings")

	err := config.Set(settings)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to save settings",
			Detail:  err.Error(),
			Cause:   err,
		})
	} else {
		if err := cache.Initialize(); err != nil {
			response.Error(util.MessageData{
				Summary: "Cache initialization error",
				Detail:  err.Error(),
				Cause:   err,
			})
		} else {
			response.Success(util.MessageData{
				Summary: "Success",
				Detail:  "Settings have been saved",
			})
		}
	}
	status.Set(status.Ready, "")
}

func (app *App) SelectDirectory(defaultDirectory string) string {
	status.Set(status.Loading, "Selecting directory")
	err, fullPath := util.ExpandPath(defaultDirectory)
	if err != nil || func() bool { _, e := os.Stat(fullPath); return os.IsNotExist(e) }() {
		fullPath, err = os.UserHomeDir()
		if err != nil {
			response.Error(util.MessageData{
				Summary: "Failed to retrieve user's home directory.",
			})
			return ""
		}
	}

	directory, err := application.Get().Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		SetDirectory(fullPath).
		SetTitle("Select Location").
		PromptForSingleSelection()

	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to select directory",
			Detail:  err.Error(),
			Cause:   err,
		})
	} else {
		if directory != "" {
			response.Success(util.MessageData{
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
	defer status.Set(status.Ready, "")

	err, fullPath := util.ExpandPath(defaultFile)
	startDir := filepath.Dir(fullPath)
	if err != nil || func() bool { _, e := os.Stat(startDir); return os.IsNotExist(e) }() {
		fullPath, err = os.UserHomeDir()
		if err != nil {
			response.Error(util.MessageData{
				Summary: "Failed to retrieve user's home directory.",
			})
			return ""
		}
		startDir = fullPath
	}

	// On macOS, file filters prevent selecting files without extensions
	// so we skip filters entirely on darwin
	fileDialog := application.Get().Dialog.OpenFile().
		CanChooseFiles(true).
		SetDirectory(startDir).
		SetTitle("Select File")
	if runtime.GOOS != "darwin" {
		fileDialog = fileDialog.AddFilter("All Files", "*")
	}

	file, err := fileDialog.PromptForSingleSelection()

	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to select file",
			Detail:  err.Error(),
			Cause:   err,
		})
	} else {
		if file != "" {
			response.Success(util.MessageData{
				Summary: "File selected",
			})
		} else {
			file = defaultFile
		}
	}
	return file
}

func (app *App) RefreshModels() {
	clearConsole()
	status.Set(status.Loading, "Refreshing models")
	err := modelManager.ReloadModels()
	if err == nil {
		response.Success(util.MessageData{
			Summary: "Models refreshed",
		})
		status.Set(status.Ready, "")
	} else {
		status.Set(status.Warning, "Some of the selected engines didn't start")
	}

}

//</editor-fold>

// <editor-fold desc="Profiles">

func (app *App) GetProfiles() string {
	manager := profile.GetManager()

	profiles, err := manager.GetAllProfiles()
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to get profiles",
			Detail:  err.Error(),
			Cause:   err,
		})
		return "[]"
	}

	profilesJSON, err := json.Marshal(profiles)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to serialize profiles",
			Detail:  err.Error(),
			Cause:   err,
		})
		return "[]"
	}

	return string(profilesJSON)
}

func (app *App) GetProfile(profileID string) string {
	manager := profile.GetManager()

	prof, err := manager.GetProfile(profileID)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to get profile",
			Detail:  err.Error(),
			Cause:   err,
		})
		return "{}"
	}

	profileJSON, err := json.Marshal(prof)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to serialize profile",
			Detail:  err.Error(),
			Cause:   err,
		})
		return "{}"
	}

	return string(profileJSON)
}

func (app *App) CreateProfile(id, name, description string) string {
	manager := profile.GetManager()

	prof, err := manager.CreateProfile(id, name, description)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to create profile",
			Detail:  err.Error(),
			Cause:   err,
		})
		return ""
	}

	response.Success(util.MessageData{
		Summary: "Profile created successfully",
		Detail:  fmt.Sprintf("Profile '%s' has been created", name),
	})

	profileJSON, _ := json.Marshal(prof)
	return string(profileJSON)
}

func (app *App) DeleteProfile(profileID string) {
	manager := profile.GetManager()

	err := manager.DeleteProfile(profileID)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to delete profile",
			Detail:  err.Error(),
			Cause:   err,
		})
	} else {
		response.Success(util.MessageData{
			Summary: "Profile deleted successfully",
		})
	}
}

func (app *App) GetProfileVoices(profileID string) string {
	manager := profile.GetManager()

	voiceProfile, err := manager.GetProfile(profileID)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to get profile",
			Detail:  err.Error(),
			Cause:   err,
		})
		return "{}"
	}

	voicesJSON, err := json.Marshal(voiceProfile.Voices)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to get profile voices",
			Detail:  err.Error(),
			Cause:   err,
		})
		return "{}"
	}

	return string(voicesJSON)
}

func (app *App) SaveProfileVoices(profileID string, voices string) {
	manager := profile.GetManager()

	voiceProfile, err := manager.GetProfile(profileID)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to get profile",
			Detail:  err.Error(),
			Cause:   err,
		})
		return
	}

	var voicesMap map[string]*util.CharacterVoice
	err = json.Unmarshal([]byte(voices), &voicesMap)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to parse voices",
			Detail:  err.Error(),
			Cause:   err,
		})
		return
	}

	voiceProfile.Voices = voicesMap

	err = manager.SaveProfile(voiceProfile)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to save profile voices",
			Detail:  err.Error(),
			Cause:   err,
		})
	} else {
		response.Success(util.MessageData{
			Summary: "Successfully saved profile voices",
		})
	}

}

func (app *App) GetConfigSchema() string {
	schema, err := config.GetConfigSchema()
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to get config schema",
			Detail:  err.Error(),
			Cause:   err,
		})
		return "{}"
	}

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to serialize config schema",
			Detail:  err.Error(),
			Cause:   err,
		})
		return "{}"
	}

	return string(schemaJSON)
}

func (app *App) GetProfileSettings(profileID string) string {
	manager := profile.GetManager()

	selectedProfile, err := manager.GetProfile(profileID)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to get profile",
			Detail:  err.Error(),
			Cause:   err,
		})
		return "{}"
	}

	settings := selectedProfile.GetSettings()
	if settings == nil {
		return "{}"
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to serialize profile settings",
			Detail:  err.Error(),
			Cause:   err,
		})
		return "{}"
	}

	return string(settingsJSON)
}

func (app *App) SaveProfileSettings(profileID, settingsJSON string) {
	manager := profile.GetManager()

	selectedProfile, err := manager.GetProfile(profileID)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to get profile",
			Detail:  err.Error(),
			Cause:   err,
		})
		return
	}

	var settings profile.ProfileSettings
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to parse settings",
			Detail:  err.Error(),
			Cause:   err,
		})
		return
	}

	selectedProfile.SetSettings(&settings)

	if err := manager.SaveProfile(selectedProfile); err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to save profile settings",
			Detail:  err.Error(),
			Cause:   err,
		})
	} else {
		response.Success(util.MessageData{
			Summary: "Profile settings saved successfully",
		})
	}
}

func (app *App) GetProfileSettingsSchema() string {
	schema, err := config.GetProfileSettingsSchema()
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to get profile settings schema",
			Detail:  err.Error(),
			Cause:   err,
		})
		return "{}"
	}

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to serialize profile settings schema",
			Detail:  err.Error(),
			Cause:   err,
		})
		return "{}"
	}

	return string(schemaJSON)
}

func (app *App) GetEnginesForProfile(profileID string) string {
	manager := profile.GetManager()

	selectedProfile, err := manager.GetProfile(profileID)
	if err != nil {
		return app.GetEngines()
	}

	profileToggles := selectedProfile.GetModelToggles()
	if profileToggles == nil || len(profileToggles) == 0 {
		return app.GetEngines()
	}

	engineToggles := make(map[string]map[string]bool)
	for key, value := range profileToggles {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		engineID := parts[0]
		modelID := parts[1]

		if _, exists := engineToggles[engineID]; !exists {
			engineToggles[engineID] = make(map[string]bool)
		}
		engineToggles[engineID][modelID] = value
	}

	allEngines := modelManager.GetAllEngines()
	var filteredEngines []map[string]interface{}

	for _, eng := range allEngines {
		filteredModels := make(map[string]interface{})

		if engineModels, exists := engineToggles[eng.ID]; exists {
			for modelID, model := range eng.Models {
				if engineModels[modelID] {
					filteredModels[modelID] = map[string]interface{}{
						"id":     model.ID,
						"name":   model.Name,
						"engine": model.Engine,
					}
				}
			}
		}

		if len(filteredModels) > 0 {
			filteredEngines = append(filteredEngines, map[string]interface{}{
				"id":     eng.ID,
				"name":   eng.Name,
				"models": filteredModels,
			})
		}
	}

	jsonData, err := json.Marshal(filteredEngines)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to get engines for profile",
			Detail:  err.Error(),
			Cause:   err,
		})
		return "[]"
	}

	return string(jsonData)
}

// </editor-fold>

// <editor-fold desc="Server Management">

func getCliExecutablePath() (string, error) {
	err, executablePath := util.ExpandPath(os.Args[0])
	if err != nil {
		return "", response.Err("%w", err)
	}

	executableDirectory := filepath.Dir(executablePath)
	cliExecutableName := "nstudio-cli"
	if runtime.GOOS == "windows" {
		cliExecutableName = "nstudio-cli.exe"
	} else if runtime.GOOS == "darwin" {
		cliExecutableName = "nstudio-cli-osx"
	}

	cliExecutablePath := filepath.Join(executableDirectory, cliExecutableName)

	if _, err := os.Stat(cliExecutablePath); os.IsNotExist(err) {
		return "", response.Err("CLI executable not found at: %s", cliExecutablePath)
	}

	return cliExecutablePath, nil
}

func (app *App) StartDaemonServer(mode string, port int, host string, configFile string) string {
	cliExecutablePath, err := getCliExecutablePath()
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to locate CLI executable",
			Detail:  err.Error(),
			Cause:   err,
		})
		return jsonError("Failed to locate CLI executable", err.Error())
	}

	arguments := []string{"-mode", mode}

	if port > 0 {
		arguments = append(arguments, "-port", fmt.Sprintf("%d", port))
	}

	if host != "" {
		arguments = append(arguments, "-host", host)
	}

	if configFile != "" {
		arguments = append(arguments, "-config", configFile)
	}

	launchDaemon := func() error {
		command := exec.Command(cliExecutablePath, arguments...)
		process.HideCommandLine(command)

		if output, err := command.CombinedOutput(); err != nil {
			return response.Err("%s: %w", strings.TrimSpace(string(output)), err)
		}
		return nil
	}

	if mode == "http" {
		status.Set(status.Loading, "Starting the server; loading its voice models can take a while")
		err = enginehost.StartDaemon(launchDaemon)
		status.Set(status.Ready, "")
	} else {
		err = launchDaemon()
		time.Sleep(1 * time.Second)
	}

	if errors.Is(err, enginehost.ErrStartCancelled) {
		response.Info(util.MessageData{Summary: "Server start cancelled"})
		return app.GetServerStatus()
	}
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to start server",
			Detail:  err.Error(),
			Cause:   err,
		})
		return jsonError("Failed to start server", err.Error())
	}

	response.Success(util.MessageData{
		Summary: "Server started",
		Detail:  fmt.Sprintf("Server started successfully in %s mode on %s:%d", mode, host, port),
	})

	return app.GetServerStatus()
}

func (app *App) StopDaemonServer() string {
	cliExecutablePath, err := getCliExecutablePath()
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to locate CLI executable",
			Detail:  err.Error(),
			Cause:   err,
		})
		return jsonError("Failed to locate CLI executable", err.Error())
	}

	var output []byte
	status.Set(status.Loading, "Stopping server")
	err = enginehost.StopDaemon(func() error {
		command := exec.Command(cliExecutablePath, "-stop")
		process.HideCommandLine(command)

		var stopErr error
		output, stopErr = command.CombinedOutput()
		return stopErr
	})
	status.Set(status.Ready, "")

	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to stop server",
			Detail:  string(output),
		})
		return jsonError("Failed to stop server", string(output))
	}

	response.Success(util.MessageData{
		Summary: "Server stopped",
		Detail:  string(output),
	})

	return app.GetServerStatus()
}

func (app *App) GetServerStatus() string {
	cliExecutablePath, err := getCliExecutablePath()
	if err != nil {
		return jsonError("Failed to locate CLI executable", err.Error())
	}

	command := exec.Command(cliExecutablePath, "-status")
	process.HideCommandLine(command)

	output, err := command.CombinedOutput()

	outputString := string(output)

	serverStatus := map[string]interface{}{
		"running":           false,
		"output":            outputString,
		"error":             "",
		"pid":               0,
		"version":           "",
		"uptime":            "",
		"processedMessages": 0,
	}

	if err == nil {
		serverStatus["running"] = true
		parseStatusOutput(outputString, serverStatus)
	} else {
		if strings.Contains(outputString, "Server is not running") {
			serverStatus["running"] = false
			serverStatus["output"] = "Server is not running"
		} else {
			serverStatus["error"] = err.Error()
			if outputString != "" {
				serverStatus["output"] = outputString
			}
		}
	}

	jsonData, _ := json.Marshal(serverStatus)
	return string(jsonData)
}

func (app *App) GetServerLogs() string {
	logFilePath := filepath.Join(os.TempDir(), "narration-studio-daemon.log")

	fileInfo, err := os.Stat(logFilePath)
	if os.IsNotExist(err) {
		result := map[string]interface{}{
			"logs": "Log file does not exist. Server may not be running or hasn't created logs yet.",
		}
		jsonData, _ := json.Marshal(result)
		return string(jsonData)
	}
	if err != nil {
		return jsonError("Failed to access log file", err.Error())
	}

	file, err := os.Open(logFilePath)
	if err != nil {
		return jsonError("Failed to open log file", err.Error())
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)

	const maxScanTokenSize = 1024 * 1024 // 1MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return jsonError("Failed to read log file", err.Error())
	}

	maxLines := 1000
	startIndex := 0
	if len(lines) > maxLines {
		startIndex = len(lines) - maxLines
	}

	tailedLines := lines[startIndex:]
	logsContent := strings.Join(tailedLines, "\n")

	result := map[string]interface{}{
		"logs":      logsContent,
		"totalSize": fileInfo.Size(),
		"lineCount": len(lines),
		"showing":   len(tailedLines),
	}

	jsonData, _ := json.Marshal(result)
	return string(jsonData)
}

func (app *App) GetConfigFilePath() string {
	return config.GetCurrentConfigFile()
}

func (app *App) GenerateServerCommand(mode string, port int, host string, configFile string) string {
	cliExecutablePath, err := getCliExecutablePath()
	if err != nil {
		return ""
	}

	arguments := []string{cliExecutablePath, "-mode", mode}

	if port > 0 {
		arguments = append(arguments, "-port", fmt.Sprintf("%d", port))
	}

	if host != "" {
		arguments = append(arguments, "-host", host)
	}

	if configFile != "" {
		arguments = append(arguments, "-config", configFile)
	}

	return strings.Join(arguments, " ")
}

func parseStatusOutput(output string, status map[string]interface{}) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.Contains(line, "Server is running (PID:") {
			var pid int
			fmt.Sscanf(line, "🟢 Server is running (PID: %d)", &pid)
			status["pid"] = pid
		}

		if strings.HasPrefix(line, "Version:") {
			version := strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
			status["version"] = version
		}

		if strings.HasPrefix(line, "Uptime:") {
			uptime := strings.TrimSpace(strings.TrimPrefix(line, "Uptime:"))
			status["uptime"] = uptime
		}

		if strings.HasPrefix(line, "Processed Messages:") {
			var messages int
			fmt.Sscanf(line, "Processed Messages: %d", &messages)
			status["processedMessages"] = messages
		}
	}
}

func jsonError(summary string, detail string) string {
	result := map[string]interface{}{
		"error":   summary,
		"detail":  detail,
		"running": false,
	}
	jsonData, _ := json.Marshal(result)
	return string(jsonData)
}

// </editor-fold>

// <editor-fold desc="Common">
func (app *App) GetEngines() string {
	engines := modelManager.GetAllEngines()

	jsonData, err := json.Marshal(engines)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to get engines",
			Detail:  err.Error(),
			Cause:   err,
		})
		return ""
	}

	return string(jsonData)
}

func (app *App) GetModelVoices(engine string, model string) string {
	voices, err := modelManager.GetModelVoices(engine, model)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to get voices",
			Detail:  err.Error(),
			Cause:   err,
		})
	}

	jsonData, err := json.Marshal(voices)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to get voices",
			Detail:  err.Error(),
			Cause:   err,
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

func (app *App) IsPiperGPUAvailable() bool {
	return native.IsGPUAvailable()
}

func (app *App) PiperFetchAvailableModels() string {
	status.Set(status.Loading, "Fetching Piper models")
	models, err := piper.FetchAvailableModels()
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to fetch Piper models",
			Detail:  err.Error(),
			Cause:   err,
		})
		status.Set(status.Ready, "")
		return "[]"
	}

	jsonData, err := json.Marshal(models)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to marshal Piper models",
			Detail:  err.Error(),
			Cause:   err,
		})
		status.Set(status.Ready, "")
		return "[]"
	}

	status.Set(status.Ready, "")
	return string(jsonData)
}

func (app *App) PiperDownloadModel(modelJSON string) string {
	var model piper.AvailableModel
	if err := json.Unmarshal([]byte(modelJSON), &model); err != nil {
		response.Error(util.MessageData{
			Summary: "Invalid model payload",
			Detail:  err.Error(),
			Cause:   err,
		})
		return err.Error()
	}

	status.Set(status.Loading, "Downloading "+model.Name)
	if err := piper.DownloadModel(model); err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to download model " + model.Name,
			Detail:  err.Error(),
			Cause:   err,
		})
		status.Set(status.Ready, "")
		return err.Error()
	}

	modelManager.ReloadModels()
	response.Success(util.MessageData{
		Summary: "Downloaded " + model.Name,
	})
	status.Set(status.Ready, "")
	return ""
}

func (app *App) PiperDeleteModel(modelID string) string {
	status.Set(status.Loading, "Deleting Piper model")
	if err := piper.DeleteModel(modelID); err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to delete model " + modelID,
			Detail:  err.Error(),
			Cause:   err,
		})
		status.Set(status.Ready, "")
		return err.Error()
	}

	modelManager.ReloadModels()
	response.Success(util.MessageData{
		Summary: "Deleted " + modelID,
	})
	status.Set(status.Ready, "")
	return ""
}

//</editor-fold>

// <editor-fold desc="Engine Packs">

type enginePackListing struct {
	Packs     []pack.AvailablePack `json:"packs"`
	Installed []pack.InstalledPack `json:"installed"`
}

func (app *App) EnginePackFetchAvailable() string {
	status.Set(status.Loading, "Fetching engine packs")
	defer status.Set(status.Ready, "")

	listing := enginePackListing{Packs: []pack.AvailablePack{}, Installed: []pack.InstalledPack{}}
	var firstErr error

	if localPath := config.GetSettings().LocalEnginePacksPath; localPath != "" {
		localPacks, err := pack.FetchAvailableFromPath(localPath)
		if err != nil {
			firstErr = err
		}
		listing.Packs = append(listing.Packs, localPacks...)
	}

	remotePacks, err := pack.FetchAvailable()
	if err != nil && firstErr == nil {
		firstErr = err
	}
	listing.Packs = append(listing.Packs, remotePacks...)

	installedPacks, err := pack.ScanInstalled()
	if err != nil && firstErr == nil {
		firstErr = err
	}
	listing.Installed = append(listing.Installed, installedPacks...)

	if len(listing.Packs) == 0 && firstErr != nil {
		response.Error(util.MessageData{
			Summary: "Failed to fetch engine packs",
			Detail:  firstErr.Error(),
			Cause:   firstErr,
		})
	}

	jsonData, err := json.Marshal(listing)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to marshal engine pack list",
			Detail:  err.Error(),
			Cause:   err,
		})
		return "{}"
	}
	return string(jsonData)
}

func (app *App) EnginePackInstall(packJSON string) string {
	var availablePack pack.AvailablePack
	if err := json.Unmarshal([]byte(packJSON), &availablePack); err != nil {
		response.Error(util.MessageData{
			Summary: "Invalid engine pack payload",
			Detail:  err.Error(),
			Cause:   err,
		})
		return err.Error()
	}
	return installEnginePack(availablePack, "Installed")
}

func installEnginePack(availablePack pack.AvailablePack, successVerb string) string {
	packLabel := availablePack.Manifest.Name + " " + availablePack.Manifest.Version
	status.Set(status.Loading, "Installing "+packLabel)
	defer status.Set(status.Ready, "")

	err := pack.Install(availablePack)
	switch {
	case err == nil:
		response.Success(util.MessageData{Summary: successVerb + " " + packLabel})
		return ""
	case errors.Is(err, pack.ErrInstallPaused):
		response.Info(util.MessageData{Summary: "Paused installing " + packLabel})
		return ""
	case errors.Is(err, pack.ErrInstallCancelled):
		response.Info(util.MessageData{Summary: "Cancelled installing " + packLabel})
		return ""
	}
	response.Error(util.MessageData{
		Summary: "Failed to install " + packLabel,
		Detail:  err.Error(),
		Cause:   err,
	})
	return err.Error()
}

func (app *App) EnginePackPauseInstall(engineID string) {
	pack.PauseInstall(engineID)
}

func (app *App) EnginePackCancelInstall(engineID string) {
	pack.CancelInstall(engineID)
}

func (app *App) EnginePackUninstall(engineID string) string {
	status.Set(status.Loading, "Uninstalling "+engineID)
	defer status.Set(status.Ready, "")

	if err := pack.Uninstall(engineID); err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to uninstall " + engineID,
			Detail:  err.Error(),
			Cause:   err,
		})
		return err.Error()
	}

	response.Success(util.MessageData{
		Summary: "Uninstalled " + engineID,
	})
	return ""
}

func (app *App) EnginePackImport(filePath string) string {
	if filePath == "" {
		return ""
	}

	if !strings.EqualFold(filepath.Ext(filePath), ".nstudio") {
		response.Error(util.MessageData{
			Summary: "Not an engine pack",
			Detail:  "Pick a .nstudio file to import.",
		})
		return "not a .nstudio file"
	}

	if _, err := os.Stat(filePath); err != nil {
		response.Error(util.MessageData{
			Summary: "File not found",
			Detail:  err.Error(),
			Cause:   err,
		})
		return err.Error()
	}

	manifest, err := pack.ReadManifestFromBundle(filePath)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Invalid engine pack",
			Detail:  err.Error(),
			Cause:   err,
		})
		return err.Error()
	}

	if manifest.OS != "" && manifest.OS != runtime.GOOS {
		detail := fmt.Sprintf("This pack is for %s, but you're on %s.", manifest.OS, runtime.GOOS)
		response.Error(util.MessageData{Summary: "Wrong platform", Detail: detail})
		return detail
	}

	if manifest.Arch != "" && manifest.Arch != runtime.GOARCH {
		detail := fmt.Sprintf("This pack is for %s, but you're on %s.", manifest.Arch, runtime.GOARCH)
		response.Error(util.MessageData{Summary: "Wrong architecture", Detail: detail})
		return detail
	}

	var bundleSize int64
	if info, statErr := os.Stat(filePath); statErr == nil {
		bundleSize = info.Size()
	}

	return installEnginePack(pack.AvailablePack{
		Manifest:   *manifest,
		BundleURL:  filePath,
		BundleSize: bundleSize,
		Local:      true,
	}, "Imported")
}

//</editor-fold>

// <editor-fold desc="Events">
func (app *App) EventSubscribe(eventName string, handler func(data interface{})) {
	eventManager.GetInstance().SubscribeToEvent(eventName, handler)
}

func (app *App) EventTrigger(eventName string, data interface{}) {
	eventManager.GetInstance().TriggerEvent(eventName, data)
}

// </editor-fold>

// <editor-fold desc="Activity Log">
func (app *App) OpenActivityLog() {
	if window, exists := application.Get().Window.GetByName("activitylog"); exists {
		window.Show()
		window.Focus()
	}
}

func (app *App) OpenActivityLogChain(chainID int) {
	app.OpenActivityLog()
	eventManager.GetInstance().EmitEvent("activity.focusChain", chainID)
}

func (app *App) GetActivityLog() string {
	entries := activity.Recent(1000)
	data, err := json.Marshal(entries)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func (app *App) GetActivityLogOlder(beforeID int, limit int) string {
	entries, err := activity.ReadOlder(uint64(beforeID), limit)
	if err != nil {
		return "[]"
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// </editor-fold>
