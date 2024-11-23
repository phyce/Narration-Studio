//go:build windows
// +build windows

package process

import (
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"syscall"
)

const (
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	STILL_ACTIVE                      = 259
)

func IsRunning(p *os.Process) bool {
	if p == nil {
		return false
	}

	handle, err := syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(p.Pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)

	var exitCode uint32
	err = syscall.GetExitCodeProcess(handle, &exitCode)
	if err != nil {
		return false
	}

	return exitCode == STILL_ACTIVE
}

func HideCommandLine(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
}
