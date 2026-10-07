package pack

import (
	"fmt"
	"nstudio/app/common/response"
)

var diskSpaceMargin int64 = 256 << 20

var freeDiskSpace = availableDiskSpace

func ensureDiskSpace(path string, neededBytes int64) error {
	freeBytes, err := freeDiskSpace(path)
	if err != nil {
		response.Err("check free disk space on %s: %w", path, err)
		return nil
	}
	neededBytes += diskSpaceMargin
	if uint64(neededBytes) > freeBytes {
		return response.Err("not enough disk space: installing needs about %s on %s, but only %s is free",
			formatBytes(neededBytes), path, formatBytes(int64(freeBytes)))
	}
	return nil
}

func formatBytes(byteCount int64) string {
	switch {
	case byteCount >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(byteCount)/(1<<30))
	case byteCount >= 1<<20:
		return fmt.Sprintf("%d MB", byteCount>>20)
	default:
		return fmt.Sprintf("%d KB", byteCount>>10)
	}
}
