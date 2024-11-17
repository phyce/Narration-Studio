//go:build !windows
// +build !windows

package process

import (
	"os"
	"syscall"
)

// isProcessRunning checks if a process is running on Unix-based systems.
func IsRunning(p *os.Process) bool {
	if p == nil {
		return false
	}

	// On Unix systems, sending signal 0 to a process is a way to check for its existence.
	// If the process does not exist, an issue will be returned.
	err := p.Signal(syscall.Signal(0))
	return err == nil
}
