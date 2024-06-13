package util

import (
	"fmt"
	"nstudio/app/config"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type CharacterMessage struct {
	Character string `json:"character"`
	Text      string `json:"text"`
	Save      bool   `json:"save"`
}

func GetKeys[KeyType comparable, ValueType any](inputMap map[KeyType]ValueType) []KeyType {
	keys := make([]KeyType, 0, len(inputMap))

	for key := range inputMap {
		keys = append(keys, key)
	}

	return keys
}

// TODO: user error generator everywhere
func GenerateError(err error, message string) error {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		return fmt.Errorf("error generating error: %v", err)
	}

	shortFile := file[strings.LastIndex(file, "/")+1:]

	newErrorMessage := fmt.Sprintf("%s:%d: %s - %v \n", shortFile, line, message, err)

	return fmt.Errorf(newErrorMessage)
}

func GenerateFilename(message CharacterMessage, index int) string {
	currentTime := time.Now()
	//datePath := currentTime.Format("2006-01-02_15-04-05")
	datePath := currentTime.Format("2006-01-02")

	text := truncateString(message.Text, 20)
	text = strings.ReplaceAll(text, " ", "_")

	filename := fmt.Sprintf("%d) %s-%s.wav", index, message.Character, text)

	basePath := *config.GetInstance().GetSetting("scriptOutputPath").String

	fullPath := filepath.Join(basePath, datePath, filename)
	return fullPath
}

func truncateString(str string, maxLength int) string {
	if len(str) > maxLength {
		return str[:maxLength]
	}
	return str
}
