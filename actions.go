package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"nstudio/app/cache"
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
	"nstudio/app/tts/modelManager"
	"nstudio/app/tts/profile"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//This is the entry point for all actions coming from the frontend

// <editor-fold desc="Sandbox">
/* TODO: combine with ProcessScript as they're mostly identical */
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
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to play script",
			Detail:  err.Error(),
		})
	} else {
		response.Success(util.MessageData{
			Summary: "Success",
			Detail:  "Generation completed",
		})
	}

	status.Set(status.Ready, "")
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
	if err != nil {
		response.Error(util.MessageData{
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
			response.Error(util.MessageData{
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
			response.Error(util.MessageData{
				Summary: "Failed to combine wav files",
				Detail:  err.Error(),
			})
			return
		}

		if outputType == OutputType.CombinedFile {
			files, err := os.ReadDir(outputPath)
			if err != nil {
				response.Error(util.MessageData{
					Summary: "Failed to read directory",
					Detail:  err.Error(),
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
		})
		return ""
	}

	data, err := json.Marshal(value)
	if err != nil {
		response.Error(util.MessageData{
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
		response.Error(util.MessageData{
			Summary: "Failed to save settings",
			Detail:  err.Error(),
		})
	} else {
		if err := cache.Initialize(); err != nil {
			response.Error(util.MessageData{
				Summary: "Cache initialization error",
				Detail:  err.Error(),
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
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to expand provided directory",
		})

		fullPath, err = os.UserHomeDir()
		if err != nil {
			response.Error(util.MessageData{
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
		response.Error(util.MessageData{
			Summary: "Failed to select directory",
			Detail:  err.Error(),
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
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to expand provided file path",
		})

		fullPath, err = os.UserHomeDir()
		if err != nil {
			response.Error(util.MessageData{
				Summary: "Failed to retrieve user's home directory.",
			})
			return ""
		}
	}

	// On macOS, file filters prevent selecting files without extensions
	// so we skip filters entirely on darwin
	var dialogOptions wailsRuntime.OpenDialogOptions
	if runtime.GOOS == "darwin" {
		dialogOptions = wailsRuntime.OpenDialogOptions{
			DefaultDirectory: filepath.Dir(fullPath),
			Title:            "Select File",
		}
	} else {
		dialogOptions = wailsRuntime.OpenDialogOptions{
			DefaultDirectory: filepath.Dir(fullPath),
			Title:            "Select File",
			Filters: []wailsRuntime.FileFilter{
				{
					DisplayName: "All Files",
					Pattern:     "*",
				},
			},
		}
	}

	file, err := wailsRuntime.OpenFileDialog(app.context, dialogOptions)

	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to select file",
			Detail:  err.Error(),
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
		})
		return "[]"
	}

	profilesJSON, err := json.Marshal(profiles)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to serialize profiles",
			Detail:  err.Error(),
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
		})
		return "{}"
	}

	profileJSON, err := json.Marshal(prof)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to serialize profile",
			Detail:  err.Error(),
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
		})
		return "{}"
	}

	voicesJSON, err := json.Marshal(voiceProfile.Voices)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to get profile voices",
			Detail:  err.Error(),
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
		})
		return
	}

	var voicesMap map[string]*util.CharacterVoice
	err = json.Unmarshal([]byte(voices), &voicesMap)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to parse voices",
			Detail:  err.Error(),
		})
		return
	}

	voiceProfile.Voices = voicesMap

	err = manager.SaveProfile(voiceProfile)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to save profile voices",
			Detail:  err.Error(),
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
		})
		return "{}"
	}

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to serialize config schema",
			Detail:  err.Error(),
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
		})
		return
	}

	var settings profile.ProfileSettings
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to parse settings",
			Detail:  err.Error(),
		})
		return
	}

	selectedProfile.SetSettings(&settings)

	if err := manager.SaveProfile(selectedProfile); err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to save profile settings",
			Detail:  err.Error(),
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
		})
		return "{}"
	}

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to serialize profile settings schema",
			Detail:  err.Error(),
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
		return "", err
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
		return "", fmt.Errorf("CLI executable not found at: %s", cliExecutablePath)
	}

	return cliExecutablePath, nil
}

func (app *App) StartDaemonServer(mode string, port int, host string, configFile string) string {
	cliExecutablePath, err := getCliExecutablePath()
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to locate CLI executable",
			Detail:  err.Error(),
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

	command := exec.Command(cliExecutablePath, arguments...)
	process.HideCommandLine(command)

	output, err := command.CombinedOutput()
	if err != nil {
		response.Error(util.MessageData{
			Summary: "Failed to start server",
			Detail:  string(output),
		})
		return jsonError("Failed to start server", string(output))
	}

	time.Sleep(1 * time.Second)

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
		})
		return jsonError("Failed to locate CLI executable", err.Error())
	}

	command := exec.Command(cliExecutablePath, "-stop")
	process.HideCommandLine(command)

	output, err := command.CombinedOutput()

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
		})
	}

	jsonData, err := json.Marshal(voices)
	if err != nil {
		response.Error(util.MessageData{
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
