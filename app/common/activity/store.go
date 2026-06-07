package activity

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"nstudio/app/config"
)

const logFileName = "activity.jsonl"
const maxLogFileSize = 5 * 1024 * 1024
const maxScanTokenSize = 1024 * 1024

var writeMutex sync.Mutex

func LogFilePath() string {
	configPath := config.GetCurrentConfigPath()
	if configPath == "" || configPath == "." {
		configPath = os.TempDir()
	}
	return filepath.Join(configPath, logFileName)
}

func writeBatch(entries []Entry) error {
	writeMutex.Lock()
	defer writeMutex.Unlock()

	logFilePath := LogFilePath()

	rotateIfTooLarge(logFilePath)

	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		_, _ = writer.Write(line)
		_ = writer.WriteByte('\n')
	}
	return writer.Flush()
}

func rotateIfTooLarge(logFilePath string) {
	fileInfo, err := os.Stat(logFilePath)
	if err != nil || fileInfo.Size() < maxLogFileSize {
		return
	}

	secondArchivePath := logFilePath + ".2"
	firstArchivePath := logFilePath + ".1"

	_ = os.Remove(secondArchivePath)
	_ = os.Rename(firstArchivePath, secondArchivePath)
	_ = os.Rename(logFilePath, firstArchivePath)
}

func loadLastID() {
	entries := readEntries(LogFilePath())
	if len(entries) > 0 {
		lastID = entries[len(entries)-1].ID
	}
}

func ReadOlder(beforeID uint64, limit int) ([]Entry, error) {
	logFilePath := LogFilePath()

	entries := readEntries(logFilePath + ".1")
	entries = append(entries, readEntries(logFilePath)...)

	var older []Entry
	for _, entry := range entries {
		if beforeID == 0 || entry.ID < beforeID {
			older = append(older, entry)
		}
	}

	if limit > 0 && len(older) > limit {
		older = older[len(older)-limit:]
	}

	return older, nil
}

func readEntries(logFilePath string) []Entry {
	file, err := os.Open(logFilePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var entries []Entry
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, maxScanTokenSize)
	scanner.Buffer(buffer, maxScanTokenSize)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry Entry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	return entries
}
