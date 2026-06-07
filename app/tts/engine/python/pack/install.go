package pack

import (
	"archive/zip"
	"encoding/json"
	"io"
	"net/http"
	"nstudio/app/common/eventManager"
	"nstudio/app/common/response"
	"nstudio/app/tts/engine/python"
	"nstudio/app/tts/modelManager"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	EngineDownloadProgressEvent = "engine.download.progress"
	EnginesUpdatedEvent         = "engines.updated"
)

type DownloadProgress struct {
	EngineID   string `json:"engineID"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Percent    int    `json:"percent"`
	Done       bool   `json:"done"`
}

type EngineUpdate struct {
	EngineID string `json:"engineID"`
	Action   string `json:"action"`
}

type progressReader struct {
	underlyingReader io.Reader
	engineID         string
	downloadedBytes  int64
	totalBytes       int64
	lastEmitTime     time.Time
	lastEmitPercent  int
}

func (progress *progressReader) Read(buffer []byte) (int, error) {
	bytesRead, err := progress.underlyingReader.Read(buffer)
	if bytesRead > 0 {
		progress.downloadedBytes += int64(bytesRead)
		progress.emitProgressIfDue(false)
	}
	return bytesRead, err
}

func (progress *progressReader) emitProgressIfDue(force bool) {
	percent := 0
	if progress.totalBytes > 0 {
		percent = int(progress.downloadedBytes * 100 / progress.totalBytes)
		if percent > 100 {
			percent = 100
		}
	}
	if !force && percent == progress.lastEmitPercent && time.Since(progress.lastEmitTime) < 100*time.Millisecond {
		return
	}
	progress.lastEmitPercent = percent
	progress.lastEmitTime = time.Now()
	eventManager.GetInstance().EmitEvent(EngineDownloadProgressEvent, DownloadProgress{
		EngineID:   progress.engineID,
		Downloaded: progress.downloadedBytes,
		Total:      progress.totalBytes,
		Percent:    percent,
		Done:       false,
	})
}

func Install(availablePack AvailablePack) error {
	if availablePack.Manifest.ID == "" {
		return response.Err("install: pack manifest has no id")
	}
	if availablePack.BundleURL == "" {
		return response.Err("install: pack %s has no bundle URL", availablePack.Manifest.ID)
	}

	rootPath, err := ensurePacksRootPath()
	if err != nil {
		return response.Err("%w", err)
	}

	archivePath := filepath.Join(rootPath, availablePack.Manifest.ID+".nstudio")
	bundlePath := filepath.Join(rootPath, availablePack.Manifest.ID)

	progress := &progressReader{engineID: availablePack.Manifest.ID, totalBytes: availablePack.BundleSize}
	progress.emitProgressIfDue(true)

	if err := downloadBundle(availablePack.BundleURL, archivePath, progress); err != nil {
		return response.Err("install: download: %w", err)
	}

	if _, err := os.Stat(bundlePath); err == nil {
		if err := os.RemoveAll(bundlePath); err != nil {
			return response.Err("install: clear stale bundle: %w", err)
		}
	}

	if err := extractZip(archivePath, bundlePath); err != nil {
		_ = os.RemoveAll(bundlePath)
		return response.Err("install: extract: %w", err)
	}
	_ = os.Remove(archivePath)

	manifest, err := readManifest(bundlePath)
	if err != nil {
		_ = os.RemoveAll(bundlePath)
		return response.Err("install: read manifest: %w", err)
	}

	if err := validateManifest(manifest, bundlePath); err != nil {
		_ = os.RemoveAll(bundlePath)
		return response.Err("install: validate: %w", err)
	}

	if runtime.GOOS != "windows" {
		interpreterPath := filepath.Join(bundlePath, filepath.FromSlash(manifest.Runtime.Python))
		_ = os.Chmod(interpreterPath, 0o755)
	}

	if err := modelManager.RegisterEngine(python.ToEngine(*manifest, bundlePath)); err != nil {
		return response.Err("install: register: %w", err)
	}
	_ = modelManager.ReloadModels()

	eventManager.GetInstance().EmitEvent(EngineDownloadProgressEvent, DownloadProgress{
		EngineID:   availablePack.Manifest.ID,
		Downloaded: progress.downloadedBytes,
		Total:      progress.totalBytes,
		Percent:    100,
		Done:       true,
	})
	eventManager.GetInstance().EmitEvent(EnginesUpdatedEvent, EngineUpdate{
		EngineID: availablePack.Manifest.ID,
		Action:   "installed",
	})

	return nil
}

func Uninstall(engineID string) error {
	if engineID == "" {
		return response.Err("uninstall: engineID is required")
	}

	if err := modelManager.UnregisterEngine(engineID); err != nil {
		return response.Err("uninstall: unregister: %w", err)
	}

	bundlePath, err := installedPackPath(engineID)
	if err != nil {
		return response.Err("%w", err)
	}

	rootPath, err := packsRootPath()
	if err != nil {
		return response.Err("%w", err)
	}
	absolutePacksRootPath, _ := filepath.Abs(rootPath)
	absoluteBundlePath, _ := filepath.Abs(bundlePath)
	if !strings.HasPrefix(absoluteBundlePath, absolutePacksRootPath+string(os.PathSeparator)) {
		return response.Err("uninstall: refusing to delete outside packs root: %s", absoluteBundlePath)
	}

	if _, err := os.Stat(bundlePath); !os.IsNotExist(err) {
		if err := os.RemoveAll(bundlePath); err != nil {
			return response.Err("uninstall: remove bundle: %w", err)
		}
	}

	eventManager.GetInstance().EmitEvent(EnginesUpdatedEvent, EngineUpdate{
		EngineID: engineID,
		Action:   "uninstalled",
	})
	return nil
}

func isLocalBundlePath(bundleSource string) bool {
	if strings.HasPrefix(bundleSource, "http://") || strings.HasPrefix(bundleSource, "https://") {
		return false
	}
	return true
}

func downloadBundle(bundleURL, destinationPath string, progress *progressReader) error {
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return response.Err("%w", err)
	}

	if isLocalBundlePath(bundleURL) {
		return copyLocalBundle(bundleURL, destinationPath, progress)
	}

	httpClient := &http.Client{Timeout: 60 * time.Minute}
	downloadResponse, err := httpClient.Get(bundleURL)
	if err != nil {
		return response.Err("%w", err)
	}
	defer downloadResponse.Body.Close()

	if downloadResponse.StatusCode != http.StatusOK {
		return response.Err("download %s: status %d", bundleURL, downloadResponse.StatusCode)
	}

	if progress.totalBytes == 0 && downloadResponse.ContentLength > 0 {
		progress.totalBytes = downloadResponse.ContentLength
	}

	progress.underlyingReader = downloadResponse.Body
	return writeThroughTemporaryFile(progress, destinationPath)
}

func copyLocalBundle(sourcePath, destinationPath string, progress *progressReader) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return response.Err("open local bundle %s: %w", sourcePath, err)
	}
	defer sourceFile.Close()

	if sourceInfo, err := sourceFile.Stat(); err == nil && progress.totalBytes == 0 {
		progress.totalBytes = sourceInfo.Size()
	}

	progress.underlyingReader = sourceFile
	return writeThroughTemporaryFile(progress, destinationPath)
}

func writeThroughTemporaryFile(sourceReader io.Reader, destinationPath string) error {
	temporaryFilePath := destinationPath + ".part"
	temporaryFile, err := os.Create(temporaryFilePath)
	if err != nil {
		return response.Err("%w", err)
	}

	if _, err := io.Copy(temporaryFile, sourceReader); err != nil {
		temporaryFile.Close()
		_ = os.Remove(temporaryFilePath)
		return response.Err("%w", err)
	}
	if err := temporaryFile.Close(); err != nil {
		_ = os.Remove(temporaryFilePath)
		return response.Err("%w", err)
	}

	if err := os.Rename(temporaryFilePath, destinationPath); err != nil {
		_ = os.Remove(temporaryFilePath)
		return response.Err("%w", err)
	}
	return nil
}

func extractZip(sourcePath, destinationPath string) error {
	zipReader, err := zip.OpenReader(sourcePath)
	if err != nil {
		return response.Err("%w", err)
	}
	defer zipReader.Close()

	if err := os.MkdirAll(destinationPath, 0o755); err != nil {
		return response.Err("%w", err)
	}

	absoluteDestinationPath, err := filepath.Abs(destinationPath)
	if err != nil {
		return response.Err("%w", err)
	}

	for _, zipEntry := range zipReader.File {
		cleanedEntryName := filepath.FromSlash(zipEntry.Name)
		if strings.Contains(cleanedEntryName, "..") || filepath.IsAbs(cleanedEntryName) {
			return response.Err("zip entry rejected (path traversal): %q", zipEntry.Name)
		}
		entryTargetPath := filepath.Join(destinationPath, cleanedEntryName)
		absoluteTargetPath, err := filepath.Abs(entryTargetPath)
		if err != nil {
			return response.Err("%w", err)
		}
		if absoluteTargetPath != absoluteDestinationPath && !strings.HasPrefix(absoluteTargetPath, absoluteDestinationPath+string(os.PathSeparator)) {
			return response.Err("zip entry rejected (escapes destination): %q", zipEntry.Name)
		}

		if zipEntry.FileInfo().IsDir() {
			if err := os.MkdirAll(entryTargetPath, 0o755); err != nil {
				return response.Err("%w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(entryTargetPath), 0o755); err != nil {
			return response.Err("%w", err)
		}

		entryReader, err := zipEntry.Open()
		if err != nil {
			return response.Err("%w", err)
		}

		entryMode := zipEntry.Mode() & 0o777
		if entryMode == 0 {
			entryMode = 0o644
		}
		targetFile, err := os.OpenFile(entryTargetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, entryMode)
		if err != nil {
			entryReader.Close()
			return response.Err("%w", err)
		}
		if _, err := io.Copy(targetFile, entryReader); err != nil {
			entryReader.Close()
			targetFile.Close()
			return response.Err("%w", err)
		}
		entryReader.Close()
		targetFile.Close()
	}
	return nil
}

func ReadManifestFromBundle(bundlePath string) (*python.Manifest, error) {
	zipReader, err := zip.OpenReader(bundlePath)
	if err != nil {
		return nil, response.Err("open bundle %s: %w", bundlePath, err)
	}
	defer zipReader.Close()

	for _, zipEntry := range zipReader.File {
		if filepath.ToSlash(zipEntry.Name) != "manifest.json" {
			continue
		}
		entryReader, err := zipEntry.Open()
		if err != nil {
			return nil, response.Err("%w", err)
		}
		rawManifest, err := io.ReadAll(entryReader)
		entryReader.Close()
		if err != nil {
			return nil, response.Err("%w", err)
		}
		var manifest python.Manifest
		if err := json.Unmarshal(rawManifest, &manifest); err != nil {
			return nil, response.Err("parse manifest.json in bundle: %w", err)
		}
		return &manifest, nil
	}
	return nil, response.Err("bundle %s has no manifest.json at its root", bundlePath)
}

func readManifest(bundlePath string) (*python.Manifest, error) {
	manifestPath := filepath.Join(bundlePath, "manifest.json")
	rawManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, response.Err("%w", err)
	}
	var manifest python.Manifest
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		return nil, response.Err("%w", err)
	}
	return &manifest, nil
}

func validateManifest(manifest *python.Manifest, bundlePath string) error {
	if manifest.SchemaVersion != 1 {
		return response.Err("unsupported manifest schemaVersion %d", manifest.SchemaVersion)
	}
	if manifest.ID == "" {
		return response.Err("manifest missing id")
	}
	if manifest.OS != "" && manifest.OS != runtime.GOOS {
		return response.Err("bundle is for %s, current OS is %s", manifest.OS, runtime.GOOS)
	}
	if manifest.Arch != "" && manifest.Arch != runtime.GOARCH {
		return response.Err("bundle is for %s, current arch is %s", manifest.Arch, runtime.GOARCH)
	}
	if manifest.Runtime.Python == "" || manifest.Runtime.Entrypoint == "" {
		return response.Err("manifest runtime is missing python or entrypoint")
	}
	interpreterPath := filepath.Join(bundlePath, filepath.FromSlash(manifest.Runtime.Python))
	if _, err := os.Stat(interpreterPath); err != nil {
		return response.Err("bundle missing python at %s: %w", interpreterPath, err)
	}
	entrypointPath := filepath.Join(bundlePath, filepath.FromSlash(manifest.Runtime.Entrypoint))
	if _, err := os.Stat(entrypointPath); err != nil {
		return response.Err("bundle missing entrypoint at %s: %w", entrypointPath, err)
	}
	return nil
}
