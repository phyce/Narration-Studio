package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	PidFileName    = "narration-studio.pid"
	LogFileName    = "narration-studio-daemon.log"
	StatusFileName = "narration-studio-status.json"
)

var (
	statusMutex       sync.Mutex
	lastWrittenStatus []byte
)

type DaemonStatusInfo struct {
	PID               int       `json:"pid"`
	Version           string    `json:"version"`
	StartTime         time.Time `json:"start_time"`
	ProcessedMessages int64     `json:"processed_messages"`
	Mode              string    `json:"mode"`
	Host              string    `json:"host"`
	Port              int       `json:"port"`
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

// WriteDaemonStatus persists the status file, skipping the write when the
// serialized status is byte-identical to what this process last wrote. Every
// field but ProcessedMessages is fixed for the life of the daemon, so without
// this an idle server rewrites the same ~250 bytes on every poll tick for as
// long as it runs. The file-exists check keeps the cache self-healing: a temp
// directory cleaner wiping the file must not leave the daemon looking dead.
func WriteDaemonStatus(status DaemonStatusInfo) error {
	statusMutex.Lock()
	defer statusMutex.Unlock()

	statusFile := GetStatusFilePath()
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling status: %v", err)
	}

	if bytes.Equal(data, lastWrittenStatus) {
		if _, err := os.Stat(statusFile); err == nil {
			return nil
		}
	}

	if err := os.WriteFile(statusFile, data, 0644); err != nil {
		return err
	}

	lastWrittenStatus = data
	return nil
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

func ServerEndpoint() (string, error) {
	running, pid, err := GetDaemonStatus()
	if err != nil {
		return "", err
	}
	if !running {
		return "", fmt.Errorf("server is not running")
	}

	status, err := ReadDaemonStatus()
	if err != nil {
		return "", err
	}
	if status.PID != pid {
		return "", fmt.Errorf("status file belongs to PID %d, not the running server (PID %d)", status.PID, pid)
	}
	if status.Mode != "http" {
		return "", fmt.Errorf("server is running in %q mode, not http", status.Mode)
	}

	return "http://" + net.JoinHostPort(dialableHost(status.Host), strconv.Itoa(status.Port)), nil
}

func dialableHost(listenHost string) string {
	listenHost = strings.Trim(listenHost, "[]")
	switch listenHost {
	case "", "0.0.0.0", "::":
		return "127.0.0.1"
	}
	return listenHost
}
