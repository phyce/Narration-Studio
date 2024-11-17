package util

import (
	"fmt"
	"nstudio/app/common/issue"
	"nstudio/app/common/response"
	"nstudio/app/common/util/fileIndex"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func GetKeys[KeyType comparable, ValueType any](inputMap map[KeyType]ValueType) []KeyType {
	keys := make([]KeyType, 0, len(inputMap))

	for key := range inputMap {
		keys = append(keys, key)
	}

	return keys
}

func GenerateFilename(message CharacterMessage, index int, outputPath string) string {
	currentTime := time.Now()
	//datePath := currentTime.Format("2006-01-02_15-04-05")
	datePath := currentTime.Format("2006-01-02")

	text := truncateString(message.Text, 20)
	text = strings.ReplaceAll(text, " ", "_")

	filename := fmt.Sprintf("%d) %s-%s.wav", index, message.Character, text)

	outputPath = filepath.Join(outputPath, datePath)
	outputPath = filepath.Join(outputPath, fileIndex.Timestamp())
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
	if InArray(runtime.GOOS, []string{"darwin", "linux"}) {
		if strings.HasPrefix(path, "~") {
			usr, err := user.Current()
			if err != nil {
				return issue.Trace(err), ""
			}
			path = filepath.Join(usr.HomeDir, path[1:])
		}
	}
	return nil, path
}

func PrepareDirectory(filePath string) error {
	err := os.MkdirAll(filePath, 0755)
	if err != nil {
		return issue.Trace(err)
	}
	return nil
}

func InArray[T comparable](needle T, haystack []T) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func truncateString(str string, maxLength int) string {
	if len(str) > maxLength {
		return str[:maxLength]
	}
	return str
}
