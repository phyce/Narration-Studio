package pack

import (
	"nstudio/app/common/response"
	"nstudio/app/tts/engine/python"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const (
	installedRecordFileName = "installed.json"
	installStateFileName    = ".install-state"
	downloadsFolderName     = ".downloads"
	reinstallFolderSuffix   = ".reinstall"
)

var (
	identifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	versionPattern    = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
)

var packsRootPath = func() (string, error) {
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

func validateIdentity(manifest python.Manifest) error {
	if !identifierPattern.MatchString(manifest.ID) {
		return response.Err("pack id %q must use only lowercase letters, digits and dashes", manifest.ID)
	}
	if !versionPattern.MatchString(manifest.Version) {
		return response.Err("pack version %q must look like 1.0.0", manifest.Version)
	}
	if manifest.Variant != "" && !identifierPattern.MatchString(manifest.Variant) {
		return response.Err("pack variant %q must use only lowercase letters, digits and dashes", manifest.Variant)
	}
	return nil
}

func enginePath(engineID string) (string, error) {
	if !identifierPattern.MatchString(engineID) {
		return "", response.Err("invalid engine id %q", engineID)
	}
	rootPath, err := packsRootPath()
	if err != nil {
		return "", response.Err("%w", err)
	}
	return filepath.Join(rootPath, engineID), nil
}

func downloadsPath() (string, error) {
	rootPath, err := packsRootPath()
	if err != nil {
		return "", response.Err("%w", err)
	}
	return filepath.Join(rootPath, downloadsFolderName), nil
}

func buildFolderName(manifest python.Manifest) string {
	if manifest.Variant == "" {
		return manifest.Version
	}
	return manifest.Version + "-" + manifest.Variant
}

func targetBuildFolderName(engineFolderPath string, manifest python.Manifest) string {
	folderName := buildFolderName(manifest)
	if record, installed := readInstalledRecord(engineFolderPath); installed && record.Folder == folderName {
		return folderName + reinstallFolderSuffix
	}
	return folderName
}

func bundleFileName(manifest python.Manifest) string {
	nameParts := []string{manifest.ID, manifest.Version, manifest.OS, manifest.Arch, manifest.Variant}
	var presentParts []string
	for _, namePart := range nameParts {
		if namePart != "" {
			presentParts = append(presentParts, namePart)
		}
	}
	return strings.Join(presentParts, "-") + ".nstudio"
}

func isDownloadOf(fileName, engineID string) bool {
	if !strings.HasPrefix(fileName, engineID+"-") {
		return false
	}
	remainder := fileName[len(engineID)+1:]
	return remainder != "" && remainder[0] >= '0' && remainder[0] <= '9'
}

func isInside(parentPath, childPath string) bool {
	absoluteParentPath, err := filepath.Abs(parentPath)
	if err != nil {
		return false
	}
	absoluteChildPath, err := filepath.Abs(childPath)
	if err != nil {
		return false
	}
	return strings.HasPrefix(absoluteChildPath, absoluteParentPath+string(os.PathSeparator))
}
