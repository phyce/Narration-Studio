package pack

import "golang.org/x/sys/windows"

func availableDiskSpace(path string) (uint64, error) {
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var freeBytesForUser, totalBytes, totalFreeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(pathPointer, &freeBytesForUser, &totalBytes, &totalFreeBytes); err != nil {
		return 0, err
	}
	return freeBytesForUser, nil
}
