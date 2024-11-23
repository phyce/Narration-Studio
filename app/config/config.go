package config

import (
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

var manager *ConfigManager

func init() {
	manager = &ConfigManager{
		config: Base{},
	}
}

func Initialize(info Info) error {
	manager.config.Info = info
	manager.filePath = filepath.Join(GetConfigPath(), "config.json")

	configFile, err := ioutil.ReadFile(manager.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			defaultConfigPath := filepath.Join(".", "config", fmt.Sprintf("config-%s-default.json", runtime.GOOS))

			configFile, err = ioutil.ReadFile(defaultConfigPath)
			if err != nil {
				return issue.Trace(err)
			}

			configPath := filepath.Dir(manager.filePath)
			if err := os.MkdirAll(configPath, 0755); err != nil {
				return issue.Trace(err)
			}

			err = ioutil.WriteFile(manager.filePath, configFile, 0644)
		} else {
			return err
		}
	}

	err = json.Unmarshal(configFile, &manager.config)
	//TODO remove Info from being saved into config file
	manager.config.Info = info

	if Debug() {
		fmt.Println("Config file location", manager.filePath)
	}

	if err != nil {
		return issue.Trace(err)
	}

	return err
}

func GetConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		issue.Panic("Failed to get user config directory", err)
	}

	return filepath.Join(configDir, manager.config.Info.Name)
}

func Export() (string, error) {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	settingsJSON, err := json.Marshal(manager.config)
	if err != nil {
		return "", issue.Trace(err)
	}

	return string(settingsJSON), nil
}

func Import(jsonString string) error {
	var newConfig Base
	err := json.Unmarshal([]byte(jsonString), &newConfig)
	if err != nil {
		return issue.Trace(err)
	}

	manager.config = newConfig

	updatedConfigs, err := json.Marshal(manager.config)
	if err != nil {
		return issue.Trace(err)
	}

	err = ioutil.WriteFile(manager.filePath, updatedConfigs, 0644)
	if err != nil {
		return issue.Trace(err)
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
		return nil, issue.Trace(fmt.Errorf("Invalid path"))
	}

	current := reflect.ValueOf(manager.config)

	for _, part := range parts {
		if current.Kind() == reflect.Ptr {
			current = current.Elem()
		}

		if current.Kind() != reflect.Struct {
			return nil, issue.Trace(fmt.Errorf("Path segment '%s' is not a struct", part))
		}

		field, found := findFieldByJSONTag(current, part)
		if !found {
			return nil, issue.Trace(fmt.Errorf("Field '%s' not found", part))
		}

		current = field
	}

	return current.Interface(), nil
}

func Set(newConfig Base) error {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	manager.config = newConfig

	updatedConfigs, err := json.Marshal(manager.config)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(manager.filePath, updatedConfigs, 0644)
}

func SetValueToPath(path string, value string) error {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return issue.Trace(fmt.Errorf("Invalid path"))
	}

	manager.lock.Lock()
	defer manager.lock.Unlock()

	current := reflect.ValueOf(&manager.config).Elem() // Get a reflect.Value of the Base struct

	for i, part := range parts {
		if current.Kind() == reflect.Ptr {
			if current.IsNil() {
				// Initialize the pointer to a new struct
				current.Set(reflect.New(current.Type().Elem()))
			}
			current = current.Elem()
		}

		if current.Kind() != reflect.Struct {
			return issue.Trace(fmt.Errorf("Path segment '%s' is not a struct", part))
		}

		field, found := findFieldByJSONTag(current, part)
		if !found {
			return issue.Trace(fmt.Errorf("Field '%s' not found", part))
		}

		if i == len(parts)-1 {
			// This is the target field to set
			if !field.CanSet() {
				return issue.Trace(fmt.Errorf("Cannot set field '%s'", part))
			}

			// Determine the type of the field
			fieldType := field.Type()

			// Create a new instance of the field's type
			newValuePtr := reflect.New(fieldType)

			// Unmarshal the JSON value into the new instance
			err := json.Unmarshal([]byte(value), newValuePtr.Interface())
			if err != nil {
				return issue.Trace(fmt.Errorf("Failed to unmarshal value: %w", err))
			}

			// Set the field to the new value (dereference the pointer)
			field.Set(newValuePtr.Elem())
			return nil
		} else {
			// Traverse to the next nested struct
			current = field
		}
	}

	return issue.Trace(fmt.Errorf("Path '%s' did not reach a field", path))
}
