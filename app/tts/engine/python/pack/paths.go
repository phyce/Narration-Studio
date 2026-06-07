package pack

import (
	"nstudio/app/common/response"
	"os"
	"path/filepath"
	"runtime"
)

func packsRootPath() (string, error) {
	if runtime.GOOS == "windows" {
		userHomePath, err := os.UserHomeDir()
		if err != nil {
			return "", response.Err("%w", err)
		}
		return filepath.Join(userHomePath, "Narration Studio", "engines"), nil
	}
	userConfigPath, err := os.UserConfigDir()
	if err != nil {
		return "", response.Err("%w", err)
	}
	return filepath.Join(userConfigPath, "Narration Studio", "engines"), nil
}

func ensurePacksRootPath() (string, error) {
	rootPath, err := packsRootPath()
	if err != nil {
		return "", response.Err("%w", err)
	}
	if err := os.MkdirAll(rootPath, 0o755); err != nil {
		return "", response.Err("create packs root %s: %w", rootPath, err)
	}
	return rootPath, nil
}

func installedPackPath(engineID string) (string, error) {
	rootPath, err := packsRootPath()
	if err != nil {
		return "", response.Err("%w", err)
	}
	return filepath.Join(rootPath, engineID), nil
}
