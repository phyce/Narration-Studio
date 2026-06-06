package pack

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func packsRootPath() (string, error) {
	if runtime.GOOS == "windows" {
		userHomePath, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(userHomePath, "Narration Studio", "engines"), nil
	}
	userConfigPath, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(userConfigPath, "Narration Studio", "engines"), nil
}

func ensurePacksRootPath() (string, error) {
	rootPath, err := packsRootPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(rootPath, 0o755); err != nil {
		return "", fmt.Errorf("create packs root %s: %w", rootPath, err)
	}
	return rootPath, nil
}

func installedPackPath(engineID string) (string, error) {
	rootPath, err := packsRootPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(rootPath, engineID), nil
}
