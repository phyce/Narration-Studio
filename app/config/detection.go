package config

import (
	"os"
	"path/filepath"
	"runtime"
)

func findEngines(config *Base) {
	if runtime.GOOS != "windows" {
		// Currently only Windows installers are supported
		return
	}

	installDir, err := getInstallDirectory()
	if err != nil || installDir == "" {
		return
	}

	piperPath := filepath.Join(installDir, "engines", "piper", "piper.exe")
	if fileExists(piperPath) {
		config.Engine.Local.Piper.Location = piperPath

		modelsDir := filepath.Join(installDir, "engines", "piper", "models")
		if dirExists(modelsDir) {
			config.Engine.Local.Piper.ModelsDirectory = modelsDir
		}
	}

	msSapi4Path := filepath.Join(installDir, "engines", "mssapi4", "sapi4out", "sapi4out.exe")
	if fileExists(msSapi4Path) {
		config.Engine.Local.MsSapi4.Location = msSapi4Path
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && info.IsDir()
}
