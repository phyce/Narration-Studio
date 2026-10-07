package activity

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nstudio/app/config"
)

// Every launch writes its own file, logs/<date>/<time>.jsonl under the config folder, so the
// Activity Log only ever shows the current session. Date folders older than
// settings.activityLogDays are deleted when a launch first writes.
const logsFolderName = "logs"
const dateFolderLayout = "2006-01-02"
const sessionFileLayout = "15-04-05"
const legacyLogFileName = "activity.jsonl"
const maxLogFileSize = 5 * 1024 * 1024
const maxScanTokenSize = 1024 * 1024

var (
	writeMutex      sync.Mutex
	sessionMutex    sync.Mutex
	sessionStarted  = time.Now()
	sessionLogsPath string
	sessionFilePath string
)

func logsPath() string {
	configPath := config.GetCurrentConfigPath()
	if configPath == "" || configPath == "." {
		configPath = os.TempDir()
	}
	return filepath.Join(configPath, logsFolderName)
}

// A config path change (tests) starts a new file in the new location.
func sessionFile() string {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	currentLogsPath := logsPath()
	if sessionFilePath == "" || sessionLogsPath != currentLogsPath {
		moveLegacyLogs(filepath.Dir(currentLogsPath), currentLogsPath)
		removeExpiredLogs(currentLogsPath, keepDays(), time.Now())
		sessionLogsPath = currentLogsPath
		sessionFilePath = createSessionFile(currentLogsPath, sessionStarted)
	}
	return sessionFilePath
}

func writtenSessionFile() string {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	return sessionFilePath
}

func keepDays() int {
	days := config.GetSettings().ActivityLogDays
	if days < 1 {
		return config.DefaultActivityLogDays
	}
	return days
}

// createSessionFile reserves logs/<date>/<time>.jsonl; two processes started in the same second
// (the desktop app and its server) get <time>-2.jsonl and so on.
func createSessionFile(logsPath string, started time.Time) string {
	dateFolderPath := filepath.Join(logsPath, started.Format(dateFolderLayout))
	baseName := started.Format(sessionFileLayout)
	firstChoice := filepath.Join(dateFolderPath, baseName+".jsonl")
	if err := os.MkdirAll(dateFolderPath, 0755); err != nil {
		return firstChoice
	}

	for attempt := 1; attempt <= 100; attempt++ {
		candidate := firstChoice
		if attempt > 1 {
			candidate = filepath.Join(dateFolderPath, fmt.Sprintf("%s-%d.jsonl", baseName, attempt))
		}
		file, err := os.OpenFile(candidate, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			_ = file.Close()
			return candidate
		}
		if !os.IsExist(err) {
			return candidate
		}
	}
	return firstChoice
}

// removeExpiredLogs deletes date folders outside the last keepDays days (today counts as one).
func removeExpiredLogs(logsPath string, keepDays int, now time.Time) {
	entries, err := os.ReadDir(logsPath)
	if err != nil {
		return
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	oldestKept := today.AddDate(0, 0, -(keepDays - 1))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		folderDate, err := time.ParseInLocation(dateFolderLayout, entry.Name(), time.Local)
		if err != nil {
			continue
		}
		if folderDate.Before(oldestKept) {
			_ = os.RemoveAll(filepath.Join(logsPath, entry.Name()))
		}
	}
}

// moveLegacyLogs moves the single activity.jsonl (+ .1/.2) that older versions kept in the config
// folder into the date folder of its last write, so it expires like the rest.
func moveLegacyLogs(configPath string, logsPath string) {
	for _, suffix := range []string{"", ".1", ".2"} {
		legacyPath := filepath.Join(configPath, legacyLogFileName+suffix)
		fileInfo, err := os.Stat(legacyPath)
		if err != nil || fileInfo.IsDir() {
			continue
		}
		dateFolderPath := filepath.Join(logsPath, fileInfo.ModTime().Format(dateFolderLayout))
		if err := os.MkdirAll(dateFolderPath, 0755); err != nil {
			continue
		}
		_ = os.Rename(legacyPath, filepath.Join(dateFolderPath, "legacy-activity"+suffix+".jsonl"))
	}
}

func writeBatch(entries []Entry) error {
	writeMutex.Lock()
	defer writeMutex.Unlock()

	logFilePath := sessionFile()

	rotateIfTooLarge(logFilePath)

	// Another launch's cleanup may have removed the folder of a session that spans several days.
	if err := os.MkdirAll(filepath.Dir(logFilePath), 0755); err != nil {
		return err
	}
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

func ReadOlder(beforeID uint64, limit int) ([]Entry, error) {
	logFilePath := writtenSessionFile()
	if logFilePath == "" {
		return nil, nil
	}

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
