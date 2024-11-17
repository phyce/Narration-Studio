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
	manager.filePath = filepath.Join(GetConfigPath(), "narrator-studio-config.json")

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
	if err != nil {
		return issue.Trace(err)
	}

	return err
}

func GetConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		issue.Panic(err)
	}

	return filepath.Join(configDir, manager.config.Info.Title)
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

	if manager.config.Settings.Debug {

	}

	if Debug() {
		fmt.Println("Wrote new config to file: ", manager.filePath)
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
		return nil, issue.Trace(fmt.Errorf("invalid path"))
	}

	current := reflect.ValueOf(manager.config)

	for _, part := range parts {
		if current.Kind() == reflect.Ptr {
			current = current.Elem()
		}

		if current.Kind() != reflect.Struct {
			return nil, issue.Trace(fmt.Errorf("path segment '%s' is not a struct", part))
		}

		field, found := findFieldByJSONTag(current, part)
		if !found {
			return nil, issue.Trace(fmt.Errorf("field '%s' not found", part))
		}

		current = field
	}

	return current.Interface(), nil
}

func Set(newConfig interface{}) error {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	// Start the update process
	err := updateStruct(reflect.ValueOf(&manager.config).Elem(), reflect.ValueOf(newConfig))
	if err != nil {
		return err
	}

	return nil
}

func SetValueToPath(path string, value string) error {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return issue.Trace(fmt.Errorf("invalid path"))
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
			return issue.Trace(fmt.Errorf("path segment '%s' is not a struct", part))
		}

		field, found := findFieldByJSONTag(current, part)
		if !found {
			return issue.Trace(fmt.Errorf("field '%s' not found", part))
		}

		if i == len(parts)-1 {
			// This is the target field to set
			if !field.CanSet() {
				return issue.Trace(fmt.Errorf("cannot set field '%s'", part))
			}

			// Determine the type of the field
			fieldType := field.Type()

			// Create a new instance of the field's type
			newValuePtr := reflect.New(fieldType)

			// Unmarshal the JSON value into the new instance
			err := json.Unmarshal([]byte(value), newValuePtr.Interface())
			if err != nil {
				return issue.Trace(fmt.Errorf("failed to unmarshal value: %w", err))
			}

			// Set the field to the new value (dereference the pointer)
			field.Set(newValuePtr.Elem())
			return nil
		} else {
			// Traverse to the next nested struct
			current = field
		}
	}

	return issue.Trace(fmt.Errorf("path '%s' did not reach a field", path))
}

// Recursive function to update structs
func updateStruct(dest, src reflect.Value) error {
	if src.Kind() == reflect.Ptr {
		src = src.Elem()
	}
	if src.Kind() != reflect.Struct {
		return fmt.Errorf("Set expects a struct or a pointer to a struct")
	}

	srcType := src.Type()
	for i := 0; i < src.NumField(); i++ {
		srcField := src.Field(i)
		srcFieldType := srcType.Field(i)

		// Get the JSON tag or use the field name
		jsonTag := srcFieldType.Tag.Get("json")
		if jsonTag == "" {
			jsonTag = srcFieldType.Name
		}

		// Find the corresponding field in dest by JSON tag
		destField, found := findFieldByJSONTag(dest, jsonTag)
		if !found {
			continue // Field not found in destination; skip
		}

		if srcField.Kind() == reflect.Struct && destField.Kind() == reflect.Struct {
			// Recursively update nested structs
			err := updateStruct(destField, srcField)
			if err != nil {
				return err
			}
		} else {
			// Only set non-zero values
			if !isZeroValue(srcField) {
				if destField.CanSet() {
					destField.Set(srcField)
				} else {
					return fmt.Errorf("cannot set field %s", destField.Type().Name())
				}
			}
		}
	}
	return nil
}

// Helper function to find a field in dest by JSON tag
func findFieldByJSONTag(dest reflect.Value, jsonTag string) (reflect.Value, bool) {
	destType := dest.Type()
	for i := 0; i < dest.NumField(); i++ {
		field := dest.Field(i)
		fieldType := destType.Field(i)
		tag := fieldType.Tag.Get("json")
		if tag == jsonTag {
			return field, true
		}
	}
	return reflect.Value{}, false
}

// Helper function to check if a value is zero
func isZeroValue(v reflect.Value) bool {
	zero := reflect.Zero(v.Type())
	return reflect.DeepEqual(v.Interface(), zero.Interface())
}

//func SetSetting(name string, value Value) error {
//	manager.lock.Lock()
//	defer manager.lock.Unlock()
//
//	manager.config[name] = value
//
//	updatedConfigs, err := json.Marshal(manager.config)
//	if err != nil {
//		return err
//	}
//
//	return ioutil.WriteFile(manager.filePath, updatedConfigs, 0644)
//}
//
//func SetConfigString(name string, value string) error {
//	return SetSetting(name, Value{String: &value})
//}
//
//func SetConfigInt(name string, value int) error {
//	return SetSetting(name, Value{Int: &value})
//}
