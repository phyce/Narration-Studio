//go:build windows
// +build windows

package process

import "os"

func IsRunning(p *os.Process) bool {
	if p == nil {
		return false
	}

	handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(p.Pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)

	var exitCode uint32
	err = syscall.GetExitCodeProcess(handle, &exitCode)
	if err != nil {
		return false
	}

	return exitCode == syscall.STILL_ACTIVE
}
