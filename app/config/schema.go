package config

import (
	"embed"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

//go:embed defaults/config.form.json
var formMetadataFS embed.FS

//go:embed defaults/config.form.darwin.json
var formMetadataDarwinFS embed.FS

type FieldMetadata struct {
	Label       string                   `json:"label,omitempty"`
	Type        string                   `json:"type,omitempty"`
	PathType    string                   `json:"pathType,omitempty"`
	Options     []map[string]interface{} `json:"options,omitempty"`
	Description string                   `json:"description,omitempty"`
	Placeholder string                   `json:"placeholder,omitempty"`
	Min         *float64                 `json:"min,omitempty"`
	Max         *float64                 `json:"max,omitempty"`
	Required    bool                     `json:"required,omitempty"`
	Dynamic     bool                     `json:"dynamic,omitempty"`
	ValueType   string                   `json:"valueType,omitempty"`
	Hidden      bool                     `json:"hidden,omitempty"`
}

type NestedFormMetadata struct {
	Label       string                        `json:"label,omitempty"`
	Type        string                        `json:"type,omitempty"`
	PathType    string                        `json:"pathType,omitempty"`
	Options     []map[string]interface{}      `json:"options,omitempty"`
	Description string                        `json:"description,omitempty"`
	Placeholder string                        `json:"placeholder,omitempty"`
	Min         *float64                      `json:"min,omitempty"`
	Max         *float64                      `json:"max,omitempty"`
	Required    bool                          `json:"required,omitempty"`
	Dynamic     bool                          `json:"dynamic,omitempty"`
	ValueType   string                        `json:"valueType,omitempty"`
	Hidden      bool                          `json:"hidden,omitempty"`
	Children    map[string]NestedFormMetadata `json:"children,omitempty"`
}

type ConfigField struct {
	Path     string         `json:"path"`
	Value    interface{}    `json:"value"`
	Metadata *FieldMetadata `json:"metadata,omitempty"`
}

type ConfigSchema struct {
	Fields []ConfigField `json:"fields"`
}

var formMetadata map[string]FieldMetadata

func LoadFormMetadata() error {
	formMetadata = make(map[string]FieldMetadata)

	data, err := formMetadataFS.ReadFile("defaults/config.form.json")
	if err != nil {
		return err
	}

	var nestedMetadata map[string]NestedFormMetadata
	if err := json.Unmarshal(data, &nestedMetadata); err != nil {
		return err
	}

	for key, value := range nestedMetadata {
		flattenNestedMetadata(key, value)
	}

	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		darwinData, err := formMetadataDarwinFS.ReadFile("defaults/config.form.darwin.json")
		if err != nil {
			return err
		}

		var darwinMetadata map[string]NestedFormMetadata
		if err := json.Unmarshal(darwinData, &darwinMetadata); err != nil {
			return err
		}

		for key, value := range darwinMetadata {
			flattenNestedMetadata(key, value)
		}
	}

	return nil
}

func GetConfigSchema() (*ConfigSchema, error) {
	if formMetadata == nil {
		if err := LoadFormMetadata(); err != nil {
			return nil, err
		}
	}

	config := Get()
	schema := &ConfigSchema{
		Fields: []ConfigField{},
	}

	flattenConfig("settings", config.Settings, &schema.Fields)
	flattenConfig("engine", config.Engine, &schema.Fields)
	flattenConfig("modelToggles", config.ModelToggles, &schema.Fields)

	return schema, nil
}

func flattenNestedMetadata(path string, nested NestedFormMetadata) {
	metadata := FieldMetadata{
		Label:       nested.Label,
		Type:        nested.Type,
		PathType:    nested.PathType,
		Options:     nested.Options,
		Description: nested.Description,
		Placeholder: nested.Placeholder,
		Min:         nested.Min,
		Max:         nested.Max,
		Required:    nested.Required,
		Dynamic:     nested.Dynamic,
		ValueType:   nested.ValueType,
		Hidden:      nested.Hidden,
	}

	formMetadata[path] = metadata

	for childKey, childValue := range nested.Children {
		childPath := path + "." + childKey
		flattenNestedMetadata(childPath, childValue)
	}
}

func flattenConfig(path string, value interface{}, fields *[]ConfigField) {
	if value == nil {
		return
	}

	if isHidden(path) {
		return
	}

	metadata := getMetadataForValue(path, value)

	reflectedValue := reflect.ValueOf(value)
	switch reflectedValue.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String:
		*fields = append(*fields, ConfigField{
			Path:     path,
			Value:    value,
			Metadata: metadata,
		})
		return
	case reflect.Struct:
		*fields = append(*fields, ConfigField{
			Path:     path,
			Value:    value,
			Metadata: metadata,
		})
		flattenStruct(path, reflectedValue, fields)
		return
	case reflect.Map:
		*fields = append(*fields, ConfigField{
			Path:     path,
			Value:    value,
			Metadata: metadata,
		})
		for _, key := range reflectedValue.MapKeys() {
			keyStr := fmt.Sprintf("%v", key.Interface())
			mapValue := reflectedValue.MapIndex(key).Interface()
			flattenConfig(path+"."+keyStr, mapValue, fields)
		}
		return
	}
}

func flattenStruct(path string, value reflect.Value, fields *[]ConfigField) {
	valueType := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := valueType.Field(i)
		fieldValue := value.Field(i)

		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		jsonName := strings.Split(jsonTag, ",")[0]
		fieldPath := path + "." + jsonName

		if !fieldValue.CanInterface() {
			continue
		}

		flattenConfig(fieldPath, fieldValue.Interface(), fields)
	}
}

func getMetadataForValue(path string, value interface{}) *FieldMetadata {
	inferred := inferMetadataFromValue(path, value)

	if formMetadata == nil {
		return inferred
	}

	formMeta, exists := formMetadata[path]
	if !exists {
		return inferred
	}

	return mergeMetadata(inferred, &formMeta)
}

func inferMetadataFromValue(path string, value interface{}) *FieldMetadata {
	label := formatLabel(path)
	fieldType := DetectFieldType(value)

	metadata := &FieldMetadata{
		Label: label,
		Type:  fieldType,
	}

	lowerPath := strings.ToLower(path)

	if strings.Contains(lowerPath, "apikey") || strings.Contains(lowerPath, "key") && strings.Contains(lowerPath, "admin") {
		metadata.Type = "password"
	} else if strings.Contains(lowerPath, "path") ||
		strings.Contains(lowerPath, "directory") ||
		strings.Contains(lowerPath, "location") {
		metadata.Type = "path"
		if strings.Contains(lowerPath, "file") || strings.Contains(lowerPath, "executable") {
			metadata.PathType = "file"
		} else {
			metadata.PathType = "directory"
		}
	}

	return metadata
}

func mergeMetadata(inferred *FieldMetadata, formMeta *FieldMetadata) *FieldMetadata {
	if inferred == nil {
		return formMeta
	}
	if formMeta == nil {
		return inferred
	}

	merged := *inferred

	if formMeta.Label != "" {
		merged.Label = formMeta.Label
	}
	if formMeta.Type != "" {
		merged.Type = formMeta.Type
	}
	if formMeta.PathType != "" {
		merged.PathType = formMeta.PathType
	}
	if formMeta.Options != nil {
		merged.Options = formMeta.Options
	}
	if formMeta.Description != "" {
		merged.Description = formMeta.Description
	}
	if formMeta.Placeholder != "" {
		merged.Placeholder = formMeta.Placeholder
	}
	if formMeta.Min != nil {
		merged.Min = formMeta.Min
	}
	if formMeta.Max != nil {
		merged.Max = formMeta.Max
	}
	if formMeta.Required {
		merged.Required = formMeta.Required
	}
	if formMeta.Dynamic {
		merged.Dynamic = formMeta.Dynamic
	}
	if formMeta.ValueType != "" {
		merged.ValueType = formMeta.ValueType
	}
	if formMeta.Hidden {
		merged.Hidden = formMeta.Hidden
	}

	return &merged
}

func isHidden(path string) bool {
	if formMetadata == nil {
		return false
	}
	if meta, exists := formMetadata[path]; exists {
		return meta.Hidden
	}
	return false
}

func formatLabel(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return path
	}

	lastPart := parts[len(parts)-1]
	var result strings.Builder
	for index, segment := range lastPart {
		if index > 0 && segment >= 'A' && segment <= 'Z' {
			result.WriteRune(' ')
		}
		result.WriteRune(segment)
	}

	str := result.String()
	if len(str) == 0 {
		return str
	}

	words := strings.Fields(str)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

func DetectFieldType(value interface{}) string {
	if value == nil {
		return "text"
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Bool:
		return "checkbox"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.String:
		return "text"
	case reflect.Struct, reflect.Map:
		return "object"
	case reflect.Slice, reflect.Array:
		return "array"
	default:
		return "text"
	}
}
