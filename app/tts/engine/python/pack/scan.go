package pack

import (
	"encoding/json"
	"os"
	"path/filepath"

	"nstudio/app/common/response"
	"nstudio/app/tts/engine/python"
)

type InstalledPack struct {
	Manifest   python.Manifest
	BundlePath string
}

func ScanInstalled() ([]InstalledPack, error) {
	rootPath, err := packsRootPath()
	if err != nil {
		return nil, response.Err("%w", err)
	}
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var installedPacks []InstalledPack
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		bundlePath := filepath.Join(rootPath, entry.Name())
		manifestPath := filepath.Join(bundlePath, "manifest.json")
		rawManifest, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var manifest python.Manifest
		if err := json.Unmarshal(rawManifest, &manifest); err != nil {
			continue
		}
		if manifest.ID == "" {
			continue
		}
		installedPacks = append(installedPacks, InstalledPack{Manifest: manifest, BundlePath: bundlePath})
	}
	return installedPacks, nil
}

func IsInstalled(engineID string) bool {
	bundlePath, err := installedPackPath(engineID)
	if err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(bundlePath, "manifest.json")); err != nil {
		return false
	}
	return true
}
