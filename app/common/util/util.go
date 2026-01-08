package util

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"nstudio/app/common/util/fileIndex"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
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

	text := TruncateString(message.Text, 20)
	text = strings.ReplaceAll(text, " ", "_")

	filename := fmt.Sprintf("%d) %s-%s.wav", index, message.Character, text)

	outputPath = filepath.Join(outputPath, datePath)
	outputPath = filepath.Join(outputPath, fileIndex.Timestamp())
	fullPath := filepath.Join(outputPath, filename)

	err := PrepareDirectory(outputPath)
	if err != nil {
		fmt.Println(err)
	}

	return fullPath
}

func ExpandPath(path string) (error, string) {
	switch runtime.GOOS {
	case "windows":
		//$WINDIR is %WINDIR%
		path = os.ExpandEnv(convertPercentVars(path))
		break
	case "darwin", "linux":
		if strings.HasPrefix(path, "~") {
			usr, err := user.Current()
			if err != nil {
				return err, ""
			}
			path = filepath.Join(usr.HomeDir, path[1:])
		}
		break
	}

	return nil, path
}

func PrepareDirectory(filePath string) error {
	err := os.MkdirAll(filePath, 0755)
	if err != nil {
		return err
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

func TruncateString(str string, maxLength int) string {
	if len(str) > maxLength {
		return str[:maxLength]
	}
	return str
}

func convertPercentVars(path string) string {
	re := regexp.MustCompile(`%([^%]+)%`)
	return re.ReplaceAllString(path, "${$1}")
}

func HashText(text string) string {
	normalized := strings.ToLower(strings.TrimSpace(text))
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])
}

func SanitizeFilename(text string) string {
	parts := strings.SplitN(text, ":", 2)
	if len(parts) == 2 {
		text = strings.TrimSpace(parts[1])
	}

	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\n", "\r", "\t"}
	sanitized := text
	for _, char := range invalid {
		sanitized = strings.ReplaceAll(sanitized, char, "_")
	}

	sanitized = strings.Join(strings.Fields(sanitized), " ")

	maxLen := 50
	if len(sanitized) > maxLen {
		sanitized = sanitized[:maxLen]
	}

	sanitized = strings.TrimRight(sanitized, " .,!?-_")

	return sanitized
}

func GetCurrentTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func FormatDuration(duration time.Duration) string {
	if duration < time.Minute {
		return fmt.Sprintf("%.0fs", duration.Seconds())
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		seconds := int(duration.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		minutes := int(duration.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		days := int(duration.Hours()) / 24
		hours := int(duration.Hours()) % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	}
}
