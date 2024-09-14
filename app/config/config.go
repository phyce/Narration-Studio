package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
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
		return err
	}
	manager.filePath = filepath.Join(filepath.Dir(executablePath), "narrator-studio-config.json")

	file, err := ioutil.ReadFile(manager.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			var defaultConfigPath string
			if runtime.GOOS == "windows" {
				defaultConfigPath = filepath.Join(".", "config", "config-windows-default.json")
			} else if runtime.GOOS == "darwin" {
				defaultConfigPath = filepath.Join(".", "config", "config-macos-default.json")
			}

			file, err = ioutil.ReadFile(defaultConfigPath)
			if err != nil {
				return err
			}

			err = ioutil.WriteFile(manager.filePath, file, 0644)
		}
		return err
	}

	err = json.Unmarshal(file, &manager.settings)
	return err
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

	return ioutil.WriteFile(manager.filePath, updatedConfigs, 0644)
}

func (manager *ConfigManager) Export() (string, error) {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	settingsJSON, err := json.Marshal(manager.settings)
	if err != nil {
		return "", err
	}

	return string(settingsJSON), nil
}

func (manager *ConfigManager) Import(jsonString string) error {
	var newConfigs map[string]Value
	err := json.Unmarshal([]byte(jsonString), &newConfigs)
	if err != nil {
		return err
	}

	manager.lock.Lock()
	defer manager.lock.Unlock()

	manager.settings = newConfigs

	updatedConfigs, err := json.Marshal(manager.settings)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(manager.filePath, updatedConfigs, 0644)
}

func (manager *ConfigManager) SetConfigString(name string, value string) error {
	return manager.SetSetting(name, Value{String: &value})
}

func (manager *ConfigManager) SetConfigInt(name string, value int) error {
	return manager.SetSetting(name, Value{Int: &value})
}
