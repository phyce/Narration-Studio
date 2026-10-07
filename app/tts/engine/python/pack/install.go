package pack

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"nstudio/app/common/eventManager"
	"nstudio/app/common/response"
	"nstudio/app/tts/engine/python"
	"nstudio/app/tts/modelManager"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

const (
	EngineDownloadProgressEvent = "engine.download.progress"
	EnginesUpdatedEvent         = "engines.updated"
)

const (
	InstallStageDownloading = "downloading"
	InstallStageExtracting  = "extracting"
	InstallStageWeights     = "weights"
	InstallStageVerifying   = "verifying"
)

var (
	folderRemovalAttempts = 10
	folderRemovalDelay    = 500 * time.Millisecond
	prepareBundle         = python.PrepareBundle
	verifyBundle          = python.VerifyBundle
)

type DownloadProgress struct {
	EngineID   string `json:"engineID"`
	Stage      string `json:"stage"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Percent    int    `json:"percent"`
	Done       bool   `json:"done"`
}

type EngineUpdate struct {
	EngineID string `json:"engineID"`
	Action   string `json:"action"`
}

type installProgress struct {
	engineID        string
	stage           string
	completedBytes  int64
	totalBytes      int64
	lastEmitTime    time.Time
	lastEmitPercent int
}

func newInstallProgress(engineID, stage string, totalBytes int64) *installProgress {
	progress := &installProgress{engineID: engineID, stage: stage, totalBytes: totalBytes}
	progress.emitIfDue(true)
	return progress
}

func (progress *installProgress) advanceTo(completedBytes, totalBytes int64) {
	progress.completedBytes = completedBytes
	if totalBytes > 0 {
		progress.totalBytes = totalBytes
	}
	progress.emitIfDue(false)
}

func (progress *installProgress) emitIfDue(force bool) {
	percent := 0
	if progress.totalBytes > 0 {
		percent = int(progress.completedBytes * 100 / progress.totalBytes)
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
		Stage:      progress.stage,
		Downloaded: progress.completedBytes,
		Total:      progress.totalBytes,
		Percent:    percent,
	})
}

type progressReader struct {
	context          context.Context
	underlyingReader io.Reader
	progress         *installProgress
}

func (reader *progressReader) Read(buffer []byte) (int, error) {
	if err := reader.context.Err(); err != nil {
		return 0, err
	}
	bytesRead, err := reader.underlyingReader.Read(buffer)
	if bytesRead > 0 {
		reader.progress.advanceTo(reader.progress.completedBytes+int64(bytesRead), 0)
	}
	return bytesRead, err
}

func Install(availablePack AvailablePack) error {
	manifest := availablePack.Manifest
	if err := validateIdentity(manifest); err != nil {
		return response.Err("install: %w", err)
	}
	if availablePack.BundleURL == "" {
		return response.Err("install: pack %s has no bundle", manifest.ID)
	}
	if !python.SupportsProtocol(manifest.Protocol) {
		return response.Err("install: %s %s needs a different version of Narration Studio (it uses engine protocol %d, this version supports %d)",
			manifest.Name, manifest.Version, manifest.Protocol, python.ProtocolVersion)
	}
	if modelManager.IsReleased() {
		return response.Err("install: %w", ErrServerRunning)
	}

	installContext, finish, err := beginInstall(manifest.ID)
	if err != nil {
		return response.Err("install: %w", err)
	}
	defer finish()

	err = installBuild(installContext, availablePack)
	if err == nil {
		return nil
	}
	switch cause := context.Cause(installContext); {
	case errors.Is(cause, ErrInstallPaused):
		return ErrInstallPaused
	case errors.Is(cause, ErrInstallCancelled):
		discardBuild(manifest)
		return ErrInstallCancelled
	}
	return err
}

func installBuild(ctx context.Context, availablePack AvailablePack) error {
	expectedManifest := availablePack.Manifest
	engineID := expectedManifest.ID

	rootPath, err := ensurePacksRootPath()
	if err != nil {
		return response.Err("%w", err)
	}
	engineFolderPath := filepath.Join(rootPath, engineID)
	if err := os.MkdirAll(engineFolderPath, 0o755); err != nil {
		return response.Err("create %s: %w", engineFolderPath, err)
	}

	previousRecord, wasInstalled := readInstalledRecord(engineFolderPath)
	buildFolderName := targetBuildFolderName(engineFolderPath, expectedManifest)
	buildPath := filepath.Join(engineFolderPath, buildFolderName)
	archiveFileName := bundleFileName(expectedManifest)
	discardOtherPartials(engineFolderPath, previousRecord, wasInstalled, buildFolderName, archiveFileName)

	if readInstallState(buildPath) == "" {
		if err := os.RemoveAll(buildPath); err != nil {
			return response.Err("clear unfinished install %s: %w", buildPath, err)
		}
		bundlePath := availablePack.BundleURL
		downloaded := !isLocalBundlePath(bundlePath)
		if downloaded {
			if bundlePath, err = downloadBundle(ctx, availablePack, rootPath, archiveFileName); err != nil {
				return err
			}
		}
		if err := extractBundle(ctx, bundlePath, buildPath, expectedManifest); err != nil {
			return err
		}
		if downloaded {
			_ = os.Remove(bundlePath)
		}
	}

	manifest, err := checkExtractedBundle(buildPath, expectedManifest)
	if err != nil {
		_ = os.RemoveAll(buildPath)
		return response.Err("%w", err)
	}

	if readInstallState(buildPath) == installStateExtracted {
		if manifest.Runtime.Prepare != nil {
			if err := prepareWeights(ctx, *manifest, buildPath); err != nil {
				return err
			}
		}
		if err := writeInstallState(buildPath, installStatePrepared); err != nil {
			return response.Err("%w", err)
		}
	}

	newInstallProgress(engineID, InstallStageVerifying, 0)
	if err := verifyBundle(ctx, *manifest, buildPath); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_ = os.RemoveAll(buildPath)
		return response.Err("%s %s failed its test run: %w", manifest.Name, manifest.Version, err)
	}

	return switchToBuild(engineFolderPath, *manifest, buildFolderName, previousRecord, wasInstalled)
}

func downloadBundle(ctx context.Context, availablePack AvailablePack, rootPath, archiveFileName string) (string, error) {
	downloadsFolderPath := filepath.Join(rootPath, downloadsFolderName)
	if err := os.MkdirAll(downloadsFolderPath, 0o755); err != nil {
		return "", response.Err("create %s: %w", downloadsFolderPath, err)
	}
	archivePath := filepath.Join(downloadsFolderPath, archiveFileName)

	var downloadedBytes int64
	if partialInfo, err := os.Stat(archivePath + ".part"); err == nil {
		downloadedBytes = partialInfo.Size()
	}
	neededBytes := max(availablePack.BundleSize-downloadedBytes, 0) + installedSizeBytes(availablePack) + weightsSizeBytes(availablePack.Manifest)
	if err := ensureDiskSpace(downloadsFolderPath, neededBytes); err != nil {
		return "", err
	}

	progress := newInstallProgress(availablePack.Manifest.ID, InstallStageDownloading, availablePack.BundleSize)
	if err := downloadResumable(ctx, availablePack.BundleURL, archivePath, availablePack.BundleSize, availablePack.SHA256, progress); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", response.Err("download %s: %w", archiveFileName, err)
	}
	return archivePath, nil
}

func installedSizeBytes(availablePack AvailablePack) int64 {
	if availablePack.Manifest.ApproxInstallSizeMB > 0 {
		return int64(availablePack.Manifest.ApproxInstallSizeMB) << 20
	}
	return 2 * availablePack.BundleSize
}

func weightsSizeBytes(manifest python.Manifest) int64 {
	if manifest.Runtime.Prepare == nil {
		return 0
	}
	return int64(manifest.Runtime.Prepare.DownloadSizeMB) << 20
}

func extractBundle(ctx context.Context, bundlePath, buildPath string, expectedManifest python.Manifest) error {
	zipReader, err := zip.OpenReader(bundlePath)
	if err != nil {
		return response.Err("open %s: %w", bundlePath, err)
	}
	defer zipReader.Close()

	var uncompressedBytes int64
	for _, zipEntry := range zipReader.File {
		uncompressedBytes += int64(zipEntry.UncompressedSize64)
	}
	if err := ensureDiskSpace(filepath.Dir(buildPath), uncompressedBytes+weightsSizeBytes(expectedManifest)); err != nil {
		return err
	}

	progress := newInstallProgress(expectedManifest.ID, InstallStageExtracting, uncompressedBytes)
	if err := extractZipEntries(ctx, zipReader, buildPath, progress); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_ = os.RemoveAll(buildPath)
		return response.Err("extract: %w", err)
	}

	manifest, err := checkExtractedBundle(buildPath, expectedManifest)
	if err != nil {
		_ = os.RemoveAll(buildPath)
		return response.Err("%w", err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(filepath.Join(buildPath, filepath.FromSlash(manifest.Runtime.Python)), 0o755)
	}
	return writeInstallState(buildPath, installStateExtracted)
}

func checkExtractedBundle(buildPath string, expectedManifest python.Manifest) (*python.Manifest, error) {
	manifest, err := readManifest(buildPath)
	if err != nil {
		return nil, response.Err("read manifest: %w", err)
	}
	if err := validateManifest(manifest, buildPath); err != nil {
		return nil, response.Err("validate: %w", err)
	}
	if manifest.ID != expectedManifest.ID || manifest.Version != expectedManifest.Version || manifest.Variant != expectedManifest.Variant {
		return nil, response.Err("the bundle contains %s %s (%s), not %s %s (%s)",
			manifest.ID, manifest.Version, manifest.Variant, expectedManifest.ID, expectedManifest.Version, expectedManifest.Variant)
	}
	return manifest, nil
}

func prepareWeights(ctx context.Context, manifest python.Manifest, buildPath string) error {
	progress := newInstallProgress(manifest.ID, InstallStageWeights, weightsSizeBytes(manifest))
	var highestDownloadedBytes atomic.Int64
	err := retryWhileProgressing(ctx, func() (bool, error) {
		previousHighest := highestDownloadedBytes.Load()
		err := prepareBundle(ctx, manifest, buildPath, func(downloaded, total int64) {
			progress.advanceTo(downloaded, total)
			if downloaded > highestDownloadedBytes.Load() {
				highestDownloadedBytes.Store(downloaded)
			}
		})
		return highestDownloadedBytes.Load() > previousHighest, err
	})
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return response.Err("download model weights: %w", err)
	}
	return nil
}

func switchToBuild(engineFolderPath string, manifest python.Manifest, buildFolderName string, previousRecord installedRecord, wasInstalled bool) error {
	if wasInstalled {
		if err := modelManager.UnregisterEngine(manifest.ID); err != nil {
			return response.Err("stop the installed version: %w", err)
		}
	}

	record := installedRecord{Version: manifest.Version, Variant: manifest.Variant, Folder: buildFolderName}
	if wasInstalled {
		record.LeftoverFolders = append(record.LeftoverFolders, previousRecord.LeftoverFolders...)
		if previousRecord.Folder != buildFolderName {
			record.LeftoverFolders = append(record.LeftoverFolders, previousRecord.Folder)
		}
	}
	if err := writeInstalledRecord(engineFolderPath, record); err != nil {
		if wasInstalled {
			registerInstalled(engineFolderPath)
		}
		return response.Err("record the new version: %w", err)
	}

	buildPath := filepath.Join(engineFolderPath, buildFolderName)
	_ = os.Remove(filepath.Join(buildPath, installStateFileName))
	if err := modelManager.RegisterEngine(python.ToEngine(manifest, buildPath)); err != nil {
		return response.Err("register: %w", err)
	}
	_ = modelManager.ReloadModels()

	if len(record.LeftoverFolders) > 0 {
		record.LeftoverFolders = removeFoldersPatiently(engineFolderPath, record.LeftoverFolders)
		if err := writeInstalledRecord(engineFolderPath, record); err != nil {
			response.Err("update %s after removing the old version: %w", engineFolderPath, err)
		}
	}

	eventManager.GetInstance().EmitEvent(EngineDownloadProgressEvent, DownloadProgress{
		EngineID: manifest.ID,
		Percent:  100,
		Done:     true,
	})
	eventManager.GetInstance().EmitEvent(EnginesUpdatedEvent, EngineUpdate{
		EngineID: manifest.ID,
		Action:   "installed",
	})
	return nil
}

func registerInstalled(engineFolderPath string) {
	installedPack, installed := readInstalledPack(engineFolderPath)
	if !installed || !installedPack.Compatible {
		return
	}
	if err := modelManager.RegisterEngine(python.ToEngine(installedPack.Manifest, installedPack.BundlePath)); err != nil {
		response.Err("register %s again: %w", installedPack.Manifest.ID, err)
	}
}

func removeFoldersPatiently(engineFolderPath string, folderNames []string) []string {
	remainingFolderNames := folderNames
	for attempt := 1; attempt <= folderRemovalAttempts && len(remainingFolderNames) > 0; attempt++ {
		if attempt > 1 {
			time.Sleep(folderRemovalDelay)
		}
		remainingFolderNames = removeFolders(engineFolderPath, remainingFolderNames, attempt == folderRemovalAttempts)
	}
	return remainingFolderNames
}

func discardOtherPartials(engineFolderPath string, activeRecord installedRecord, installed bool, keptFolderName, keptArchiveFileName string) {
	if entries, err := os.ReadDir(engineFolderPath); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || entry.Name() == keptFolderName || (installed && entry.Name() == activeRecord.Folder) {
				continue
			}
			_ = os.RemoveAll(filepath.Join(engineFolderPath, entry.Name()))
		}
	}

	downloadsFolderPath, err := downloadsPath()
	if err != nil {
		return
	}
	engineID := filepath.Base(engineFolderPath)
	if entries, err := os.ReadDir(downloadsFolderPath); err == nil {
		for _, entry := range entries {
			fileName := entry.Name()
			if entry.IsDir() || !isDownloadOf(fileName, engineID) || fileName == keptArchiveFileName || fileName == keptArchiveFileName+".part" {
				continue
			}
			_ = os.Remove(filepath.Join(downloadsFolderPath, fileName))
		}
	}
}

func discardBuild(manifest python.Manifest) {
	engineFolderPath, err := enginePath(manifest.ID)
	if err != nil {
		return
	}
	buildFolderName := targetBuildFolderName(engineFolderPath, manifest)
	if record, installed := readInstalledRecord(engineFolderPath); !installed || record.Folder != buildFolderName {
		_ = os.RemoveAll(filepath.Join(engineFolderPath, buildFolderName))
	}
	if downloadsFolderPath, err := downloadsPath(); err == nil {
		archivePath := filepath.Join(downloadsFolderPath, bundleFileName(manifest))
		_ = os.Remove(archivePath)
		_ = os.Remove(archivePath + ".part")
	}
	_ = os.Remove(engineFolderPath)
}

func Uninstall(engineID string) error {
	engineFolderPath, err := enginePath(engineID)
	if err != nil {
		return response.Err("uninstall: %w", err)
	}
	if modelManager.IsReleased() {
		return response.Err("uninstall: %w", ErrServerRunning)
	}
	if isInstalling(engineID) {
		return response.Err("uninstall: %s is being installed; cancel the install first", engineID)
	}

	if err := modelManager.UnregisterEngine(engineID); err != nil {
		return response.Err("uninstall: unregister: %w", err)
	}

	rootPath, err := packsRootPath()
	if err != nil {
		return response.Err("%w", err)
	}
	if !isInside(rootPath, engineFolderPath) {
		return response.Err("uninstall: refusing to delete outside the engine packs folder: %s", engineFolderPath)
	}
	if remaining := removeFoldersPatiently(rootPath, []string{engineID}); len(remaining) > 0 {
		return response.Err("uninstall: some files of %s are still in use; close anything using them and try again", engineID)
	}

	if downloadsFolderPath, err := downloadsPath(); err == nil {
		if entries, err := os.ReadDir(downloadsFolderPath); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && isDownloadOf(entry.Name(), engineID) {
					_ = os.Remove(filepath.Join(downloadsFolderPath, entry.Name()))
				}
			}
		}
	}

	eventManager.GetInstance().EmitEvent(EnginesUpdatedEvent, EngineUpdate{
		EngineID: engineID,
		Action:   "uninstalled",
	})
	return nil
}

func isLocalBundlePath(bundleSource string) bool {
	return !strings.HasPrefix(bundleSource, "http://") && !strings.HasPrefix(bundleSource, "https://")
}

func extractZipEntries(ctx context.Context, zipReader *zip.ReadCloser, destinationPath string, progress *installProgress) error {
	if err := os.MkdirAll(destinationPath, 0o755); err != nil {
		return response.Err("%w", err)
	}

	absoluteDestinationPath, err := filepath.Abs(destinationPath)
	if err != nil {
		return response.Err("%w", err)
	}

	for _, zipEntry := range zipReader.File {
		if err := ctx.Err(); err != nil {
			return err
		}
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
		if err := extractZipEntry(ctx, zipEntry, entryTargetPath, progress); err != nil {
			return err
		}
	}
	return nil
}

func extractZipEntry(ctx context.Context, zipEntry *zip.File, entryTargetPath string, progress *installProgress) error {
	entryReader, err := zipEntry.Open()
	if err != nil {
		return response.Err("%w", err)
	}
	defer entryReader.Close()

	entryMode := zipEntry.Mode() & 0o777
	if entryMode == 0 {
		entryMode = 0o644
	}
	targetFile, err := os.OpenFile(entryTargetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, entryMode)
	if err != nil {
		return response.Err("%w", err)
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, &progressReader{context: ctx, underlyingReader: entryReader, progress: progress}); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return response.Err("%w", err)
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
	if err := validateIdentity(*manifest); err != nil {
		return response.Err("%w", err)
	}
	if !python.SupportsProtocol(manifest.Protocol) {
		return response.Err("bundle uses engine protocol %d; this version of Narration Studio supports %d", manifest.Protocol, python.ProtocolVersion)
	}
	if !runsOnThisPlatform(*manifest) {
		return response.Err("bundle is for %s/%s, this computer is %s/%s", manifest.OS, manifest.Arch, runtime.GOOS, runtime.GOARCH)
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
