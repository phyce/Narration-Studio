package pack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"nstudio/app/tts/engine/python"
)

func FetchAvailableFromPath(localPacksPath string) ([]AvailablePack, error) {
	if localPacksPath == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(localPacksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan local engine packs path %q: %w", localPacksPath, err)
	}

	metadataPathByBaseName := make(map[string]string)
	bundlePathByBaseName := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fileName := entry.Name()
		lowerCaseFileName := strings.ToLower(fileName)
		filePath := filepath.Join(localPacksPath, fileName)
		switch {
		case strings.HasSuffix(lowerCaseFileName, ".metadata.json"):
			packBaseName := fileName[:len(fileName)-len(".metadata.json")]
			metadataPathByBaseName[packBaseName] = filePath
		case strings.HasSuffix(lowerCaseFileName, ".nstudio"):
			packBaseName := fileName[:len(fileName)-len(".nstudio")]
			bundlePathByBaseName[packBaseName] = filePath
		}
	}

	var availablePacks []AvailablePack
	for packBaseName, bundlePath := range bundlePathByBaseName {
		metadataPath, ok := metadataPathByBaseName[packBaseName]
		if !ok {
			continue
		}

		rawManifest, err := os.ReadFile(metadataPath)
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
		if manifest.OS != "" && manifest.OS != runtime.GOOS {
			continue
		}
		if manifest.Arch != "" && manifest.Arch != runtime.GOARCH {
			continue
		}

		bundleSize := int64(0)
		if bundleInfo, err := os.Stat(bundlePath); err == nil {
			bundleSize = bundleInfo.Size()
		}

		availablePacks = append(availablePacks, AvailablePack{
			Manifest:   manifest,
			BundleURL:  bundlePath,
			BundleSize: bundleSize,
			Installed:  IsInstalled(manifest.ID),
			TagName:    "local",
		})
	}

	return availablePacks, nil
}
