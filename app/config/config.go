package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"nstudio/app/common/issue"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

//go:embed defaults/config-windows-default.json
var configWindowsDefault []byte

//go:embed defaults/config-darwin-default.json
var configDarwinDefault []byte

var manager *ConfigManager

func init() {
	manager = &ConfigManager{
		config: Base{},
	}
}

func Initialize(info Info) error {
	return InitializeWithPath(info, "")
}

func InitializeWithPath(info Info, customPath string) error {
	manager.config.Info = info

	if customPath != "" {
		manager.filePath = customPath
	} else {
		manager.filePath = filepath.Join(GetDefaultConfigPath(), "config.json")
	}

	configFile, err := ioutil.ReadFile(manager.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			if runtime.GOOS == "windows" {
				configFile = configWindowsDefault
			} else {
				configFile = configDarwinDefault
			}

			configPath := filepath.Dir(manager.filePath)
			if err := os.MkdirAll(configPath, 0755); err != nil {
				return err
			}

			err = ioutil.WriteFile(manager.filePath, configFile, 0644)
		} else {
			return err
		}
	}

	err = json.Unmarshal(configFile, &manager.config)
	//TODO remove Info from being saved into defaults file
	manager.config.Info = info

	if Debug() {
		fmt.Println("Config file location", manager.filePath)
	}

	return err
}

func GetCurrentConfigPath() string {
	return filepath.Dir(manager.filePath)
}

func GetDefaultConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		issue.Panic("Failed to get user defaults directory", err)
	}

	return filepath.Join(configDir, manager.config.Info.Name)
}

func Export() (string, error) {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	settingsJSON, err := json.Marshal(manager.config)
	if err != nil {
		return "", err
	}

	return string(settingsJSON), nil
}

func Import(jsonString string) error {
	var newConfig Base
	err := json.Unmarshal([]byte(jsonString), &newConfig)
	if err != nil {
		return err
	}

	manager.config = newConfig

	updatedConfigs, err := json.MarshalIndent(manager.config, "", "\t")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(manager.filePath, updatedConfigs, 0644)
	if err != nil {
		return err
	}

	return nil
}

func Get() Base {
	return manager.config
}

func GetSettings() Settings {
	return manager.config.Settings
}

func GetEngine() Engine {
	return manager.config.Engine
}

func GetModelToggles() map[string]bool {
	return manager.config.ModelToggles
}

func GetEngineToggles() map[string]map[string]bool {
	engineToggles := make(map[string]map[string]bool)

	for key, value := range manager.config.ModelToggles {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		engine := parts[0]
		model := parts[1]

		if _, exists := engineToggles[engine]; !exists {
			engineToggles[engine] = make(map[string]bool)
		}

		engineToggles[engine][model] = value
	}

	return engineToggles
}

func GetInfo() Info {
	return manager.config.Info
}

func GetValueFromPath(path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil, fmt.Errorf("Invalid path")
	}

	current := reflect.ValueOf(manager.config)

	for _, part := range parts {
		if current.Kind() == reflect.Ptr {
			current = current.Elem()
		}

		if current.Kind() != reflect.Struct {
			return nil, fmt.Errorf("Path segment '%s' is not a struct", part)
		}

		field, found := findFieldByJSONTag(current, part)
		if !found {
			return nil, fmt.Errorf("Field '%s' not found", part)
		}

		current = field
	}

	return current.Interface(), nil
}

func Set(newConfig Base) error {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	manager.config = newConfig

	updatedConfigs, err := json.MarshalIndent(manager.config, "", "\t")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(manager.filePath, updatedConfigs, 0644)
}

func SetValueToPath(path string, value string) error {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return fmt.Errorf("Invalid path")
	}

	manager.lock.Lock()
	defer manager.lock.Unlock()

	current := reflect.ValueOf(&manager.config).Elem()

	for i, part := range parts {
		if current.Kind() == reflect.Ptr {
			if current.IsNil() {
				// Initialize the pointer to a new struct
				current.Set(reflect.New(current.Type().Elem()))
			}
			current = current.Elem()
		}

		if current.Kind() != reflect.Struct {
			return fmt.Errorf("Path segment '%s' is not a struct", part)
		}

		field, found := findFieldByJSONTag(current, part)
		if !found {
			return fmt.Errorf("Field '%s' not found", part)
		}

		if i == len(parts)-1 {
			// This is the target field to set
			if !field.CanSet() {
				return fmt.Errorf("Cannot set field '%s'", part)
			}

			newValuePtr := reflect.New(field.Type())

			// Unmarshal the JSON value into the new instance
			err := json.Unmarshal([]byte(value), newValuePtr.Interface())
			if err != nil {
				return fmt.Errorf("Failed to unmarshal value: %w", err)
			}

			// Set the field to the new value (dereference the pointer)
			field.Set(newValuePtr.Elem())

			// Save the updated config to disk
			updatedConfigs, err := json.MarshalIndent(manager.config, "", "\t")
			if err != nil {
				return err
			}

			return ioutil.WriteFile(manager.filePath, updatedConfigs, 0644)
		} else {
			// Traverse to the next nested struct
			current = field
		}
	}

	return fmt.Errorf("Path '%s' did not reach a field", path)
}
