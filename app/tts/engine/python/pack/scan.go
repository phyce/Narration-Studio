package pack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"nstudio/app/common/response"
	"nstudio/app/tts/engine/python"
)

type InstalledPack struct {
	Manifest   python.Manifest `json:"manifest"`
	BundlePath string          `json:"-"`
	Compatible bool            `json:"compatible"`
}

type installedRecord struct {
	Version         string   `json:"version"`
	Variant         string   `json:"variant"`
	Folder          string   `json:"folder"`
	LeftoverFolders []string `json:"leftoverFolders,omitempty"`
}

const (
	installStateExtracted = "extracted"
	installStatePrepared  = "prepared"
)

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
		return nil, response.Err("read engine packs folder %s: %w", rootPath, err)
	}

	var installedPacks []InstalledPack
	for _, entry := range entries {
		if !entry.IsDir() || !identifierPattern.MatchString(entry.Name()) {
			continue
		}
		if installedPack, installed := readInstalledPack(filepath.Join(rootPath, entry.Name())); installed {
			installedPacks = append(installedPacks, installedPack)
		}
	}
	return installedPacks, nil
}

func readInstalledPack(engineFolderPath string) (InstalledPack, bool) {
	record, installed := readInstalledRecord(engineFolderPath)
	if !installed {
		return InstalledPack{}, false
	}
	bundlePath := filepath.Join(engineFolderPath, record.Folder)
	manifest, err := readManifest(bundlePath)
	if err != nil || manifest.ID != filepath.Base(engineFolderPath) {
		return InstalledPack{}, false
	}
	return InstalledPack{
		Manifest:   *manifest,
		BundlePath: bundlePath,
		Compatible: python.SupportsProtocol(manifest.Protocol),
	}, true
}

func readInstalledRecord(engineFolderPath string) (installedRecord, bool) {
	rawRecord, err := os.ReadFile(filepath.Join(engineFolderPath, installedRecordFileName))
	if err != nil {
		return installedRecord{}, false
	}
	var record installedRecord
	if err := json.Unmarshal(rawRecord, &record); err != nil || !isPlainFolderName(record.Folder) {
		return installedRecord{}, false
	}
	return record, true
}

func writeInstalledRecord(engineFolderPath string, record installedRecord) error {
	rawRecord, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return response.Err("%w", err)
	}
	return writeFileAtomically(filepath.Join(engineFolderPath, installedRecordFileName), rawRecord)
}

func writeFileAtomically(filePath string, content []byte) error {
	temporaryFilePath := filePath + ".tmp"
	if err := os.WriteFile(temporaryFilePath, content, 0o644); err != nil {
		return response.Err("%w", err)
	}
	if err := os.Rename(temporaryFilePath, filePath); err != nil {
		_ = os.Remove(temporaryFilePath)
		return response.Err("%w", err)
	}
	return nil
}

func isPlainFolderName(folderName string) bool {
	return folderName != "" && folderName != "." && folderName != ".." && !strings.ContainsAny(folderName, `/\:`)
}

func readInstallState(buildPath string) string {
	state, err := os.ReadFile(filepath.Join(buildPath, installStateFileName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(state))
}

func writeInstallState(buildPath, state string) error {
	return writeFileAtomically(filepath.Join(buildPath, installStateFileName), []byte(state))
}

func hasPausedInstall(manifest python.Manifest) bool {
	if validateIdentity(manifest) != nil {
		return false
	}
	engineFolderPath, err := enginePath(manifest.ID)
	if err != nil {
		return false
	}
	buildPath := filepath.Join(engineFolderPath, targetBuildFolderName(engineFolderPath, manifest))
	if readInstallState(buildPath) != "" {
		return true
	}
	downloadsFolderPath, err := downloadsPath()
	if err != nil {
		return false
	}
	archivePath := filepath.Join(downloadsFolderPath, bundleFileName(manifest))
	for _, candidatePath := range []string{archivePath, archivePath + ".part"} {
		if _, err := os.Stat(candidatePath); err == nil {
			return true
		}
	}
	return false
}

func RemoveLeftovers() {
	rootPath, err := packsRootPath()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !identifierPattern.MatchString(entry.Name()) {
			continue
		}
		engineFolderPath := filepath.Join(rootPath, entry.Name())
		record, installed := readInstalledRecord(engineFolderPath)
		if !installed || len(record.LeftoverFolders) == 0 {
			continue
		}
		record.LeftoverFolders = removeFolders(engineFolderPath, record.LeftoverFolders, true)
		if err := writeInstalledRecord(engineFolderPath, record); err != nil {
			response.Err("update %s after removing old versions: %w", engineFolderPath, err)
		}
	}
}

func removeFolders(engineFolderPath string, folderNames []string, reportFailures bool) []string {
	var remainingFolderNames []string
	for _, folderName := range folderNames {
		folderPath := filepath.Join(engineFolderPath, folderName)
		if !isPlainFolderName(folderName) || !isInside(engineFolderPath, folderPath) {
			continue
		}
		if err := os.RemoveAll(folderPath); err != nil {
			if reportFailures {
				response.Err("remove %s: %w", folderPath, err)
			}
			remainingFolderNames = append(remainingFolderNames, folderName)
		}
	}
	return remainingFolderNames
}
