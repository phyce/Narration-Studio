package util

import (
	"fmt"
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

	return fmt.Errorf("%v\n%s", err, traceLine)
}

func GenerateFilename(message CharacterMessage, index int, outputPath string) string {
	currentTime := time.Now()
	//datePath := currentTime.Format("2006-01-02_15-04-05")
	datePath := currentTime.Format("2006-01-02")

	text := truncateString(message.Text, 20)
	text = strings.ReplaceAll(text, " ", "_")

	filename := fmt.Sprintf("%d) %s-%s.wav", index, message.Character, text)

	fullPath := filepath.Join(outputPath, datePath, filename)
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
