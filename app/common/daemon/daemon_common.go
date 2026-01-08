package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	PidFileName    = "narration-studio.pid"
	LogFileName    = "narration-studio-daemon.log"
	StatusFileName = "narration-studio-status.json"
)

var statusMutex sync.Mutex

type DaemonStatusInfo struct {
	PID               int       `json:"pid"`
	Version           string    `json:"version"`
	StartTime         time.Time `json:"start_time"`
	ProcessedMessages int64     `json:"processed_messages"`
}

func GetPidFilePath() string {
	tmpDir := os.TempDir()
	return filepath.Join(tmpDir, PidFileName)
}

func GetLogFilePath() string {
	tmpDir := os.TempDir()
	return filepath.Join(tmpDir, LogFileName)
}

func GetStatusFilePath() string {
	tmpDir := os.TempDir()
	return filepath.Join(tmpDir, StatusFileName)
}

func SetupDaemonLogger() (*os.File, error) {
	logFile, err := os.OpenFile(GetLogFilePath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("error opening log file: %v", err)
	}

	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	return logFile, nil
}

func WriteDaemonStatus(status DaemonStatusInfo) error {
	statusMutex.Lock()
	defer statusMutex.Unlock()

	statusFile := GetStatusFilePath()
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling status: %v", err)
	}

	return os.WriteFile(statusFile, data, 0644)
}

func ReadDaemonStatus() (*DaemonStatusInfo, error) {
	statusFile := GetStatusFilePath()
	data, err := os.ReadFile(statusFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("status file not found")
		}
		return nil, fmt.Errorf("error reading status file: %v", err)
	}

	var status DaemonStatusInfo
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("error parsing status file: %v", err)
	}

	return &status, nil
}
