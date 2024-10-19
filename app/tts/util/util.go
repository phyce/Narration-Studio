package util

import (
	"fmt"
	"nstudio/app/common/response"
	"os"
	"os/user"
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

func TraceError(err error) error {
	if err == nil {
		return nil
	}

	_, file, line, ok := runtime.Caller(1)
	if !ok {
		panic(fmt.Errorf("TraceError failed: %v", err))
	}

	shortFile := shortFileName(file)
	traceLine := fmt.Sprintf("%s:%d", shortFile, line)

	result := fmt.Errorf("%v\n%s", err, traceLine)

	return result
}

func GenerateFilename(message CharacterMessage, index int, outputPath string) string {
	currentTime := time.Now()
	//datePath := currentTime.Format("2006-01-02_15-04-05")
	datePath := currentTime.Format("2006-01-02")

	text := truncateString(message.Text, 20)
	text = strings.ReplaceAll(text, " ", "_")

	filename := fmt.Sprintf("%d) %s-%s.wav", index, message.Character, text)

	outputPath = filepath.Join(outputPath, datePath)
	outputPath = filepath.Join(outputPath, timestamp)
	fullPath := filepath.Join(outputPath, filename)

	err := PrepareDirectory(outputPath)
	if err != nil {
		response.Error(response.Data{
			Summary: "Failed to prepare directory: " + outputPath,
			Detail:  err.Error(),
		})
	}

	return fullPath
}

func ExpandPath(path string) (error, string) {
	if strings.HasPrefix(path, "~") {
		usr, err := user.Current()
		if err != nil {
			return TraceError(err), ""
		}
		path = filepath.Join(usr.HomeDir, path[1:])
	}
	return nil, path
}

func PrepareDirectory(filePath string) error {
	err := os.MkdirAll(filePath, 0755)
	if err != nil {
		return TraceError(err)
	}
	return nil
}

func shortFileName(fullPath string) string {
	lastSlash := strings.LastIndex(fullPath, "/")
	if lastSlash == -1 {
		lastSlash = strings.LastIndex(fullPath, "\\")
	}
	if lastSlash == -1 {
		return fullPath
	}
	return fullPath[lastSlash+1:]
}

func truncateString(str string, maxLength int) string {
	if len(str) > maxLength {
		return str[:maxLength]
	}
	return str
}
