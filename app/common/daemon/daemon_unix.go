//go:build !windows
// +build !windows

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// IsDaemonChild returns true if this process is running as a daemon child
func IsDaemonChild() bool {
	return os.Getenv("NSTUDIO_DAEMON_CHILD") == "1"
}

func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix-like systems, send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func GetDaemonStatus() (bool, int, error) {
	pidFile := GetPidFilePath()
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil // Not running, no PID file
		}
		return false, 0, fmt.Errorf("error reading PID file: %v", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		return false, 0, fmt.Errorf("invalid PID in file: %v", err)
	}

	if IsProcessRunning(pid) {
		return true, pid, nil
	}

	os.Remove(pidFile)
	return false, 0, nil
}

func StopDaemon() error {
	isRunning, pid, err := GetDaemonStatus()
	if err != nil {
		return fmt.Errorf("error checking daemon status: %v", err)
	}

	if !isRunning {
		return fmt.Errorf("daemon is not running")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("error finding process: %v", err)
	}

	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("error sending SIGTERM: %v", err)
	}

	os.Remove(GetPidFilePath())
	os.Remove(GetStatusFilePath())

	return nil
}

func Daemonize() (int, error) {
	logFile := GetLogFilePath()
	fmt.Printf("Server log will be written to: %s\n", logFile)

	isRunning, pid, err := GetDaemonStatus()
	if err != nil {
		return 0, fmt.Errorf("error checking server status: %v", err)
	}

	if isRunning {
		return pid, fmt.Errorf("server is already running with PID %d", pid)
	}

	execPath, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("error getting executable path: %v", err)
	}

	args := []string{}
	for i, arg := range os.Args {
		if i == 0 {
			continue
		}
		args = append(args, arg)
	}

	fmt.Printf("Starting background process: %s %s\n", execPath, strings.Join(args, " "))

	command := exec.Command(execPath, args...)

	logFileHandle, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 0, fmt.Errorf("error creating log file: %v", err)
	}

	command.Stdout = logFileHandle
	command.Stderr = logFileHandle
	command.Stdin = nil

	// Set environment variable to mark this as daemon child
	command.Env = append(os.Environ(), "NSTUDIO_DAEMON_CHILD=1")

	command.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	err = command.Start()
	if err != nil {
		logFileHandle.Close()
		return 0, fmt.Errorf("error starting background process: %v", err)
	}

	pid = command.Process.Pid
	fmt.Printf("Started background process with PID: %d\n", pid)

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logFileHandle.WriteString(fmt.Sprintf("\n=== DAEMON STARTED AT %s ===\n", timestamp))
	logFileHandle.WriteString(fmt.Sprintf("Parent PID: %d, Daemon PID: %d\n", os.Getpid(), pid))
	logFileHandle.WriteString(fmt.Sprintf("Command: %s %s\n", execPath, strings.Join(args, " ")))
	logFileHandle.WriteString("=== DAEMON OUTPUT BELOW ===\n")
	logFileHandle.Close()

	pidFile := GetPidFilePath()
	pidString := strconv.Itoa(pid)
	err = os.WriteFile(pidFile, []byte(pidString), 0644)
	if err != nil {
		return 0, fmt.Errorf("error writing PID file: %v", err)
	}

	go func() {
		fmt.Printf("Parent process %d: Monitoring daemon process %d\n", os.Getpid(), pid)

		processState, err := command.Process.Wait()

		exitLogFile, logErr := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if logErr == nil {
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			exitMessage := fmt.Sprintf("\n=== DAEMON EXITED AT %s ===\n", timestamp)
			if err != nil {
				exitMessage += fmt.Sprintf("Exit error: %v\n", err)
			}
			if processState != nil {
				exitMessage += fmt.Sprintf("Exit code: %d\n", processState.ExitCode())
				exitMessage += fmt.Sprintf("Success: %t\n", processState.Success())
			}

			exitMessage += "=== END DAEMON SESSION ===\n\n"
			exitLogFile.WriteString(exitMessage)
			exitLogFile.Close()
		}

		os.Remove(pidFile)
		os.Remove(GetStatusFilePath())
		fmt.Printf("Parent process %d: Daemon process %d exited, cleaned up files\n", os.Getpid(), pid)
	}()

	time.Sleep(100 * time.Millisecond)

	if !IsProcessRunning(pid) {
		return 0, fmt.Errorf("daemon process %d died immediately after starting", pid)
	}

	return pid, nil
}
