package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"nstudio/app/tts/util"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var instance *ConfigManager
var once sync.Once

func GetInstance() *ConfigManager {
	once.Do(func() {
		instance = &ConfigManager{
			settings: make(map[string]Value),
		}
	})
	return instance
}

func (manager *ConfigManager) Initialize() error {
	executablePath, err := os.Executable()
	if err != nil {
		return util.TraceError(err)
	}
	manager.filePath = filepath.Join(filepath.Dir(executablePath), "narrator-studio-config.json")

	configFile, err := ioutil.ReadFile(manager.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			var defaultConfigPath string
			if runtime.GOOS == "windows" {
				defaultConfigPath = filepath.Join(".", "config", "config-windows-default.json")
			} else if runtime.GOOS == "darwin" {
				defaultConfigPath = filepath.Join(".", "config", "config-macos-default.json")
			}

			configFile, err = ioutil.ReadFile(defaultConfigPath)
			if err != nil {
				return util.TraceError(err)
			}

			err = ioutil.WriteFile(manager.filePath, configFile, 0644)
		}
		return err
	}

	err = json.Unmarshal(configFile, &manager.settings)
	if err != nil {
		return util.TraceError(err)
	}

	return err
}

func (manager *ConfigManager) Export() (string, error) {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	settingsJSON, err := json.Marshal(manager.settings)
	if err != nil {
		return "", util.TraceError(err)
	}

	return string(settingsJSON), nil
}

func (manager *ConfigManager) Import(jsonString string) error {
	var newConfigs map[string]Value
	err := json.Unmarshal([]byte(jsonString), &newConfigs)
	if err != nil {
		return util.TraceError(err)
	}

	manager.lock.Lock()
	defer manager.lock.Unlock()

	manager.settings = newConfigs

	updatedConfigs, err := json.Marshal(manager.settings)
	if err != nil {
		return util.TraceError(err)
	}

	err = ioutil.WriteFile(manager.filePath, updatedConfigs, 0644)
	if err != nil {
		return util.TraceError(err)
	}

	return nil
}

func (manager *ConfigManager) GetSettings() map[string]Value {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	settings := make(map[string]Value)
	for key, value := range manager.settings {
		settings[key] = value
	}
	return settings
}

func (manager *ConfigManager) GetSetting(name string) *Value {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	if value, exists := manager.settings[name]; exists {
		return &value
	}

	return nil
}

func (manager *ConfigManager) SetSetting(name string, value Value) error {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	manager.settings[name] = value

	updatedConfigs, err := json.Marshal(manager.settings)
	if err != nil {
		return err
	}

	//fmt.Println("manager.filePath")
	//fmt.Println(manager.filePath)

	return ioutil.WriteFile(manager.filePath, updatedConfigs, 0644)
}

func (manager *ConfigManager) SetConfigString(name string, value string) error {
	return manager.SetSetting(name, Value{String: &value})
}

func (manager *ConfigManager) SetConfigInt(name string, value int) error {
	return manager.SetSetting(name, Value{Int: &value})
}

// TODO this might need to go elsewhere
func (manager *ConfigManager) GetModelToggles() map[string]map[string]bool {
	//Seems like for random reason sometimes modelToggles comes out as String?
	//Not sure what the hell is going on
	engineTogglesRaw := manager.GetSetting("modelToggles").Raw

	//engineTogglesString := *manager.GetSetting("modelToggles").String
	//fmt.Println("engineTogglesString:", engineTogglesString)

	engineToggles2D := make(map[string]map[string]bool)

	var togglesMap map[string]bool
	err := json.Unmarshal([]byte(engineTogglesRaw), &togglesMap)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		return engineToggles2D
	}

	for key, value := range togglesMap {
		parts := strings.Split(key, ":")
		if len(parts) != 2 {
			continue
		}

		if _, exists := engineToggles2D[parts[0]]; !exists {
			engineToggles2D[parts[0]] = make(map[string]bool)
		}

		engineToggles2D[parts[0]][parts[1]] = value
	}

	return engineToggles2D
}
