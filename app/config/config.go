package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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
	manager.filePath = filepath.Join(filepath.Dir(executablePath), "config.json")

	file, err := ioutil.ReadFile(manager.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File does not exist, create an empty file.
			return ioutil.WriteFile(manager.filePath, []byte("{}"), 0644)
		}
		return err
	}
	fmt.Println("File")
	fmt.Println(string(file))

	err = json.Unmarshal(file, &manager.settings)
	fmt.Println("File")
	fmt.Println(string(file))
	fmt.Println("Settings")
	fmt.Println(manager.settings)
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
	fmt.Println("\n\n\n\n\n\nOutput:")
	fmt.Println(jsonString)
	err := json.Unmarshal([]byte(jsonString), &newConfigs)
	if err != nil {
		return err
	}

	manager.lock.Lock()
	defer manager.lock.Unlock()

	manager.settings = newConfigs

	// Persist the new configuration to the file.
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
