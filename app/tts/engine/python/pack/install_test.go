package pack

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"nstudio/app/tts/engine/enginetest"
	"nstudio/app/tts/engine/python"
	"nstudio/app/tts/modelManager"
)

const testEngineID = "testpack"

func TestMain(m *testing.M) {
	os.Exit(enginetest.RunWithTemporaryConfig(m))
}

func usePacksRoot(t *testing.T) string {
	t.Helper()
	modelManager.Initialize(true)
	rootPath := t.TempDir()
	previousPacksRootPath := packsRootPath
	packsRootPath = func() (string, error) { return rootPath, nil }
	t.Cleanup(func() {
		packsRootPath = previousPacksRootPath
		_ = modelManager.UnregisterEngine(testEngineID)
	})
	return rootPath
}

func stubRunnerSteps(t *testing.T, verifyErr error) *atomic.Int32 {
	t.Helper()
	var verifyCount atomic.Int32
	previousPrepareBundle, previousVerifyBundle := prepareBundle, verifyBundle
	prepareBundle = func(context.Context, python.Manifest, string, func(int64, int64)) error { return nil }
	verifyBundle = func(context.Context, python.Manifest, string) error {
		verifyCount.Add(1)
		return verifyErr
	}
	t.Cleanup(func() { prepareBundle, verifyBundle = previousPrepareBundle, previousVerifyBundle })
	return &verifyCount
}

func shortenRetries(t *testing.T) {
	t.Helper()
	previousFirstDelay, previousLongestDelay, previousStall, previousAttempts := firstRetryDelay, longestRetryDelay, downloadStallTimeout, attemptsWithoutProgressAllowed
	firstRetryDelay, longestRetryDelay, downloadStallTimeout, attemptsWithoutProgressAllowed = 10*time.Millisecond, 20*time.Millisecond, 300*time.Millisecond, 3
	t.Cleanup(func() {
		firstRetryDelay, longestRetryDelay, downloadStallTimeout, attemptsWithoutProgressAllowed = previousFirstDelay, previousLongestDelay, previousStall, previousAttempts
	})
}

func testManifest(version string) python.Manifest {
	return python.Manifest{
		SchemaVersion: 1,
		Protocol:      python.ProtocolVersion,
		ID:            testEngineID,
		Name:          "Test Pack",
		Version:       version,
		Variant:       "cpu",
		Type:          "local",
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		Runtime:       python.Runtime{Python: "python/python.exe", Entrypoint: "runner.py"},
		Models:        []python.ManifestModel{{ID: "voice-model", Name: "Voice", Voices: []python.ManifestVoice{{ID: "alice"}}}},
	}
}

type fakeBundle struct {
	path   string
	bytes  []byte
	sha256 string
}

func buildFakeBundle(t *testing.T, manifest python.Manifest, weightsSize int) fakeBundle {
	t.Helper()
	bundlePath := filepath.Join(t.TempDir(), bundleFileName(manifest))
	bundleFile, err := os.Create(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	zipWriter := zip.NewWriter(bundleFile)
	rawManifest, _ := json.Marshal(manifest)
	weights := make([]byte, weightsSize)
	_, _ = rand.Read(weights)
	for name, content := range map[string][]byte{
		"manifest.json":      rawManifest,
		"python/python.exe":  []byte("not really python"),
		"runner.py":          []byte("print('hello')"),
		"models/weights.bin": weights,
	} {
		entryWriter, err := zipWriter.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entryWriter.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	bundleFile.Close()

	bundleBytes, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	checksum := sha256.Sum256(bundleBytes)
	return fakeBundle{path: bundlePath, bytes: bundleBytes, sha256: hex.EncodeToString(checksum[:])}
}

type bundleServer struct {
	content          []byte
	chunkDelay       time.Duration
	firstRequestDrop int
	firstRequestHang int
	failStatus       int

	requestCount  atomic.Int32
	mutex         sync.Mutex
	rangeRequests []string
}

func (server *bundleServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	requestNumber := server.requestCount.Add(1)
	if server.failStatus != 0 {
		writer.WriteHeader(server.failStatus)
		return
	}

	offset := 0
	if rangeHeader := request.Header.Get("Range"); rangeHeader != "" {
		server.mutex.Lock()
		server.rangeRequests = append(server.rangeRequests, rangeHeader)
		server.mutex.Unlock()
		fmt.Sscanf(rangeHeader, "bytes=%d-", &offset)
	}
	body := server.content[offset:]
	writer.Header().Set("Content-Length", fmt.Sprint(len(body)))
	if offset > 0 {
		writer.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, len(server.content)-1, len(server.content)))
		writer.WriteHeader(http.StatusPartialContent)
	} else {
		writer.WriteHeader(http.StatusOK)
	}

	sendLimit := len(body)
	if requestNumber == 1 && server.firstRequestDrop > 0 {
		sendLimit = min(sendLimit, server.firstRequestDrop)
	}
	if requestNumber == 1 && server.firstRequestHang > 0 {
		sendLimit = min(sendLimit, server.firstRequestHang)
	}

	const chunkSize = 32 * 1024
	for sent := 0; sent < sendLimit; sent += chunkSize {
		if _, err := writer.Write(body[sent:min(sent+chunkSize, sendLimit)]); err != nil {
			return
		}
		writer.(http.Flusher).Flush()
		if server.chunkDelay > 0 {
			select {
			case <-time.After(server.chunkDelay):
			case <-request.Context().Done():
				return
			}
		}
	}
	if requestNumber == 1 && server.firstRequestHang > 0 {
		<-request.Context().Done()
	}
}

func (server *bundleServer) recordedRanges() []string {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	return append([]string(nil), server.rangeRequests...)
}

func serveBundle(t *testing.T, server *bundleServer) string {
	t.Helper()
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)
	return httpServer.URL + "/bundle.nstudio"
}

func catalogPack(manifest python.Manifest, bundle fakeBundle, url string) AvailablePack {
	return AvailablePack{
		Manifest:   manifest,
		BundleURL:  url,
		BundleSize: int64(len(bundle.bytes)),
		SHA256:     bundle.sha256,
		Compatible: true,
	}
}

func installedVersion(t *testing.T) (installedRecord, bool) {
	t.Helper()
	enginePathValue, err := enginePath(testEngineID)
	if err != nil {
		t.Fatal(err)
	}
	return readInstalledRecord(enginePathValue)
}

func isRegistered(engineID string) bool {
	for _, registeredEngine := range modelManager.GetAllEngines() {
		if registeredEngine.ID == engineID {
			return true
		}
	}
	return false
}

func folderNames(t *testing.T, path string) []string {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func installFromServer(t *testing.T, version string, server *bundleServer) (AvailablePack, fakeBundle) {
	t.Helper()
	manifest := testManifest(version)
	bundle := buildFakeBundle(t, manifest, 64*1024)
	server.content = bundle.bytes
	availablePack := catalogPack(manifest, bundle, serveBundle(t, server))
	if err := Install(availablePack); err != nil {
		t.Fatalf("Install %s: %v", version, err)
	}
	return availablePack, bundle
}

func TestInstallDownloadsChecksAndRegisters(t *testing.T) {
	rootPath := usePacksRoot(t)
	verifyCount := stubRunnerSteps(t, nil)

	installFromServer(t, "1.0.0", &bundleServer{})

	record, installed := installedVersion(t)
	if !installed || record.Folder != "1.0.0-cpu" || record.Version != "1.0.0" || record.Variant != "cpu" {
		t.Fatalf("installed record = %+v (installed %v), want 1.0.0-cpu", record, installed)
	}
	if verifyCount.Load() != 1 {
		t.Errorf("test run happened %d times, want 1", verifyCount.Load())
	}
	if !isRegistered(testEngineID) {
		t.Error("engine is not registered after install")
	}
	if names := folderNames(t, filepath.Join(rootPath, testEngineID)); strings.Join(names, ",") != "1.0.0-cpu,installed.json" {
		t.Errorf("engine folder holds %v", names)
	}
	if names := folderNames(t, filepath.Join(rootPath, downloadsFolderName)); len(names) != 0 {
		t.Errorf("downloads left behind: %v", names)
	}
	if _, err := os.Stat(filepath.Join(rootPath, testEngineID, "1.0.0-cpu", installStateFileName)); !os.IsNotExist(err) {
		t.Error("install state file left in the installed build")
	}

	installedPacks, err := ScanInstalled()
	if err != nil || len(installedPacks) != 1 || installedPacks[0].Manifest.Version != "1.0.0" || !installedPacks[0].Compatible {
		t.Errorf("ScanInstalled = %+v, %v", installedPacks, err)
	}
}

func TestUpdateReplacesInstalledVersion(t *testing.T) {
	rootPath := usePacksRoot(t)
	stubRunnerSteps(t, nil)

	installFromServer(t, "1.0.0", &bundleServer{})
	installFromServer(t, "1.1.0", &bundleServer{})

	record, _ := installedVersion(t)
	if record.Folder != "1.1.0-cpu" || len(record.LeftoverFolders) != 0 {
		t.Fatalf("installed record = %+v, want 1.1.0-cpu with no leftovers", record)
	}
	if names := folderNames(t, filepath.Join(rootPath, testEngineID)); strings.Join(names, ",") != "1.1.0-cpu,installed.json" {
		t.Errorf("engine folder holds %v, want only the new build", names)
	}
	if !isRegistered(testEngineID) {
		t.Error("engine is not registered after the update")
	}
}

func TestFailedTestRunKeepsInstalledVersion(t *testing.T) {
	rootPath := usePacksRoot(t)
	stubRunnerSteps(t, nil)
	installFromServer(t, "1.0.0", &bundleServer{})

	stubRunnerSteps(t, errors.New("no audible audio"))
	manifest := testManifest("1.1.0")
	bundle := buildFakeBundle(t, manifest, 1024)
	err := Install(catalogPack(manifest, bundle, serveBundle(t, &bundleServer{content: bundle.bytes})))
	if err == nil || !strings.Contains(err.Error(), "failed its test run") {
		t.Fatalf("Install error = %v, want a failed test run", err)
	}

	record, _ := installedVersion(t)
	if record.Folder != "1.0.0-cpu" {
		t.Errorf("installed record = %+v, want 1.0.0-cpu kept", record)
	}
	if names := folderNames(t, filepath.Join(rootPath, testEngineID)); strings.Join(names, ",") != "1.0.0-cpu,installed.json" {
		t.Errorf("engine folder holds %v, want the failed build removed", names)
	}
	if !isRegistered(testEngineID) {
		t.Error("the installed version was unregistered by a failed update")
	}
}

func TestReinstallingTheSameVersionUsesASecondFolder(t *testing.T) {
	rootPath := usePacksRoot(t)
	stubRunnerSteps(t, nil)

	installFromServer(t, "1.0.0", &bundleServer{})
	installFromServer(t, "1.0.0", &bundleServer{})
	record, _ := installedVersion(t)
	if record.Folder != "1.0.0-cpu"+reinstallFolderSuffix {
		t.Fatalf("installed record = %+v, want the reinstall folder", record)
	}

	installFromServer(t, "1.0.0", &bundleServer{})
	record, _ = installedVersion(t)
	if record.Folder != "1.0.0-cpu" {
		t.Fatalf("installed record = %+v, want the plain folder again", record)
	}
	if names := folderNames(t, filepath.Join(rootPath, testEngineID)); strings.Join(names, ",") != "1.0.0-cpu,installed.json" {
		t.Errorf("engine folder holds %v", names)
	}
}

func waitForPartialDownload(t *testing.T, rootPath string, manifest python.Manifest, minimumBytes int64) {
	t.Helper()
	partialPath := filepath.Join(rootPath, downloadsFolderName, bundleFileName(manifest)+".part")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if partialInfo, err := os.Stat(partialPath); err == nil && partialInfo.Size() >= minimumBytes {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("download never reached %d bytes", minimumBytes)
}

func installInBackground(availablePack AvailablePack) chan error {
	result := make(chan error, 1)
	go func() { result <- Install(availablePack) }()
	return result
}

func awaitInstall(t *testing.T, result chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(20 * time.Second):
		t.Fatal("install did not return")
		return nil
	}
}

func TestPausedDownloadResumesWhereItStopped(t *testing.T) {
	rootPath := usePacksRoot(t)
	stubRunnerSteps(t, nil)

	manifest := testManifest("1.0.0")
	bundle := buildFakeBundle(t, manifest, 2*1024*1024)
	server := &bundleServer{content: bundle.bytes, chunkDelay: 5 * time.Millisecond}
	availablePack := catalogPack(manifest, bundle, serveBundle(t, server))

	result := installInBackground(availablePack)
	waitForPartialDownload(t, rootPath, manifest, 256*1024)
	if !PauseInstall(testEngineID) {
		t.Fatal("PauseInstall found no running install")
	}
	if err := awaitInstall(t, result); !errors.Is(err, ErrInstallPaused) {
		t.Fatalf("Install error = %v, want ErrInstallPaused", err)
	}
	if !hasPausedInstall(manifest) {
		t.Error("paused install is not reported as paused")
	}
	if _, installed := installedVersion(t); installed {
		t.Error("paused install is reported as installed")
	}

	if err := Install(availablePack); err != nil {
		t.Fatalf("resumed Install: %v", err)
	}
	ranges := server.recordedRanges()
	if len(ranges) != 1 || ranges[0] == "bytes=0-" {
		t.Errorf("resume sent Range headers %v, want one non-zero offset", ranges)
	}
	if record, installed := installedVersion(t); !installed || record.Folder != "1.0.0-cpu" {
		t.Errorf("installed record after resume = %+v", record)
	}
	if hasPausedInstall(manifest) {
		t.Error("finished install is still reported as paused")
	}
}

func TestCancelDiscardsTheNewBuild(t *testing.T) {
	rootPath := usePacksRoot(t)
	stubRunnerSteps(t, nil)

	manifest := testManifest("1.0.0")
	bundle := buildFakeBundle(t, manifest, 2*1024*1024)
	availablePack := catalogPack(manifest, bundle, serveBundle(t, &bundleServer{content: bundle.bytes, chunkDelay: 5 * time.Millisecond}))

	result := installInBackground(availablePack)
	waitForPartialDownload(t, rootPath, manifest, 128*1024)
	CancelInstall(testEngineID)
	if err := awaitInstall(t, result); !errors.Is(err, ErrInstallCancelled) {
		t.Fatalf("Install error = %v, want ErrInstallCancelled", err)
	}

	if names := folderNames(t, filepath.Join(rootPath, downloadsFolderName)); len(names) != 0 {
		t.Errorf("cancelled download left %v", names)
	}
	if _, err := os.Stat(filepath.Join(rootPath, testEngineID)); !os.IsNotExist(err) {
		t.Error("cancelled first install left an engine folder")
	}
	if hasPausedInstall(manifest) {
		t.Error("cancelled install is reported as paused")
	}
}

func TestCancelDuringUpdateKeepsInstalledVersion(t *testing.T) {
	rootPath := usePacksRoot(t)
	stubRunnerSteps(t, nil)
	installFromServer(t, "1.0.0", &bundleServer{})

	manifest := testManifest("1.1.0")
	bundle := buildFakeBundle(t, manifest, 2*1024*1024)
	result := installInBackground(catalogPack(manifest, bundle, serveBundle(t, &bundleServer{content: bundle.bytes, chunkDelay: 5 * time.Millisecond})))
	waitForPartialDownload(t, rootPath, manifest, 128*1024)
	CancelInstall(testEngineID)
	if err := awaitInstall(t, result); !errors.Is(err, ErrInstallCancelled) {
		t.Fatalf("Install error = %v, want ErrInstallCancelled", err)
	}

	if record, _ := installedVersion(t); record.Folder != "1.0.0-cpu" {
		t.Errorf("installed record = %+v, want 1.0.0 kept", record)
	}
	if !isRegistered(testEngineID) {
		t.Error("cancelling an update unregistered the installed version")
	}
}

func TestDownloadReconnectsAfterDrop(t *testing.T) {
	usePacksRoot(t)
	stubRunnerSteps(t, nil)
	shortenRetries(t)

	server := &bundleServer{firstRequestDrop: 40 * 1024}
	installFromServer(t, "1.0.0", server)

	if ranges := server.recordedRanges(); len(ranges) != 1 || ranges[0] != "bytes=40960-" {
		t.Errorf("reconnect sent Range headers %v, want bytes=40960-", ranges)
	}
}

func TestStalledDownloadReconnects(t *testing.T) {
	usePacksRoot(t)
	stubRunnerSteps(t, nil)
	shortenRetries(t)

	server := &bundleServer{firstRequestHang: 32 * 1024}
	installFromServer(t, "1.0.0", server)

	if server.requestCount.Load() != 2 {
		t.Errorf("server saw %d requests, want a reconnect after the stall", server.requestCount.Load())
	}
}

func TestDownloadGivesUpWhenNothingArrives(t *testing.T) {
	rootPath := usePacksRoot(t)
	stubRunnerSteps(t, nil)
	shortenRetries(t)

	manifest := testManifest("1.0.0")
	bundle := buildFakeBundle(t, manifest, 1024)
	server := &bundleServer{failStatus: http.StatusBadGateway}
	err := Install(catalogPack(manifest, bundle, serveBundle(t, server)))
	if err == nil || !strings.Contains(err.Error(), "gave up after 3 attempts") {
		t.Fatalf("Install error = %v, want giving up after 3 attempts", err)
	}
	if _, installed := installedVersion(t); installed {
		t.Error("failed download is reported as installed")
	}
	if names := folderNames(t, filepath.Join(rootPath, testEngineID)); len(names) != 0 {
		t.Errorf("failed download left %v in the engine folder", names)
	}
}

func TestMissingBundleFailsWithoutRetrying(t *testing.T) {
	usePacksRoot(t)
	stubRunnerSteps(t, nil)
	shortenRetries(t)

	manifest := testManifest("1.0.0")
	bundle := buildFakeBundle(t, manifest, 1024)
	server := &bundleServer{failStatus: http.StatusNotFound}
	if err := Install(catalogPack(manifest, bundle, serveBundle(t, server))); err == nil {
		t.Fatal("Install succeeded against a 404")
	}
	if server.requestCount.Load() != 1 {
		t.Errorf("server saw %d requests, want no retries for a 404", server.requestCount.Load())
	}
}

func TestCorruptedDownloadIsRejected(t *testing.T) {
	rootPath := usePacksRoot(t)
	stubRunnerSteps(t, nil)

	manifest := testManifest("1.0.0")
	bundle := buildFakeBundle(t, manifest, 1024)
	availablePack := catalogPack(manifest, bundle, serveBundle(t, &bundleServer{content: bundle.bytes}))
	availablePack.SHA256 = strings.Repeat("0", 64)

	err := Install(availablePack)
	if !errors.Is(err, errDownloadCorrupted) {
		t.Fatalf("Install error = %v, want errDownloadCorrupted", err)
	}
	if names := folderNames(t, filepath.Join(rootPath, downloadsFolderName)); len(names) != 0 {
		t.Errorf("corrupted download left %v", names)
	}
}

func TestNotEnoughDiskSpace(t *testing.T) {
	rootPath := usePacksRoot(t)
	stubRunnerSteps(t, nil)
	previousFreeDiskSpace := freeDiskSpace
	freeDiskSpace = func(string) (uint64, error) { return 10 << 20, nil }
	t.Cleanup(func() { freeDiskSpace = previousFreeDiskSpace })

	manifest := testManifest("1.0.0")
	bundle := buildFakeBundle(t, manifest, 1024)
	server := &bundleServer{content: bundle.bytes}
	err := Install(catalogPack(manifest, bundle, serveBundle(t, server)))
	if err == nil || !strings.Contains(err.Error(), "not enough disk space") {
		t.Fatalf("Install error = %v, want not enough disk space", err)
	}
	if server.requestCount.Load() != 0 {
		t.Error("download started despite the disk being full")
	}
	if names := folderNames(t, filepath.Join(rootPath, testEngineID)); len(names) != 0 {
		t.Errorf("engine folder holds %v", names)
	}
}

func TestUnfinishedExtractionIsNotInstalled(t *testing.T) {
	rootPath := usePacksRoot(t)
	stubRunnerSteps(t, nil)

	manifest := testManifest("1.0.0")
	unfinishedBuildPath := filepath.Join(rootPath, testEngineID, "1.0.0-cpu")
	if err := os.MkdirAll(unfinishedBuildPath, 0o755); err != nil {
		t.Fatal(err)
	}
	rawManifest, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(unfinishedBuildPath, "manifest.json"), rawManifest, 0o644); err != nil {
		t.Fatal(err)
	}

	if installedPacks, _ := ScanInstalled(); len(installedPacks) != 0 {
		t.Fatalf("half-extracted build is reported as installed: %+v", installedPacks)
	}
	RemoveLeftovers()

	bundle := buildFakeBundle(t, manifest, 1024)
	if err := Install(catalogPack(manifest, bundle, serveBundle(t, &bundleServer{content: bundle.bytes}))); err != nil {
		t.Fatalf("Install over a half-extracted build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(unfinishedBuildPath, "models", "weights.bin")); err != nil {
		t.Errorf("build was not extracted again: %v", err)
	}
}

func TestIncompatibleProtocolIsRefused(t *testing.T) {
	usePacksRoot(t)
	stubRunnerSteps(t, nil)

	manifest := testManifest("1.0.0")
	manifest.Protocol = python.ProtocolVersion + 1
	bundle := buildFakeBundle(t, manifest, 1024)
	server := &bundleServer{content: bundle.bytes}
	err := Install(catalogPack(manifest, bundle, serveBundle(t, server)))
	if err == nil || !strings.Contains(err.Error(), "needs a different version of Narration Studio") {
		t.Fatalf("Install error = %v, want a protocol refusal", err)
	}
	if server.requestCount.Load() != 0 {
		t.Error("incompatible pack was downloaded")
	}
}

func TestWeightsDownloadRetriesWhileItProgresses(t *testing.T) {
	usePacksRoot(t)
	stubRunnerSteps(t, nil)
	shortenRetries(t)

	var attempts atomic.Int32
	prepareBundle = func(_ context.Context, _ python.Manifest, _ string, report func(int64, int64)) error {
		attempt := attempts.Add(1)
		report(int64(attempt)*100, 1000)
		if attempt < 4 {
			return errors.New("connection reset")
		}
		return nil
	}

	manifest := testManifest("1.0.0")
	manifest.Runtime.Prepare = &python.Prepare{DownloadSizeMB: 1, Source: "example.com"}
	bundle := buildFakeBundle(t, manifest, 1024)
	if err := Install(catalogPack(manifest, bundle, serveBundle(t, &bundleServer{content: bundle.bytes}))); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if attempts.Load() != 4 {
		t.Errorf("prepare ran %d times, want 4", attempts.Load())
	}
}

func TestWeightsDownloadGivesUpWithoutProgressAndCanResume(t *testing.T) {
	rootPath := usePacksRoot(t)
	stubRunnerSteps(t, nil)
	shortenRetries(t)

	prepareBundle = func(_ context.Context, _ python.Manifest, _ string, report func(int64, int64)) error {
		report(0, 1000)
		return errors.New("network unreachable")
	}

	manifest := testManifest("1.0.0")
	manifest.Runtime.Prepare = &python.Prepare{DownloadSizeMB: 1, Source: "example.com"}
	bundle := buildFakeBundle(t, manifest, 1024)
	server := &bundleServer{content: bundle.bytes}
	availablePack := catalogPack(manifest, bundle, serveBundle(t, server))
	err := Install(availablePack)
	if err == nil || !strings.Contains(err.Error(), "network unreachable") {
		t.Fatalf("Install error = %v, want the weights failure", err)
	}
	if !hasPausedInstall(manifest) {
		t.Error("an install that stopped at the weights is not resumable")
	}

	prepareBundle = func(context.Context, python.Manifest, string, func(int64, int64)) error { return nil }
	if err := Install(availablePack); err != nil {
		t.Fatalf("resumed Install: %v", err)
	}
	if server.requestCount.Load() != 1 {
		t.Errorf("bundle was downloaded %d times, want once", server.requestCount.Load())
	}
	if names := folderNames(t, filepath.Join(rootPath, testEngineID)); strings.Join(names, ",") != "1.0.0-cpu,installed.json" {
		t.Errorf("engine folder holds %v", names)
	}
}

func TestLocalBundleInstallsWithoutDownloading(t *testing.T) {
	rootPath := usePacksRoot(t)
	stubRunnerSteps(t, nil)

	manifest := testManifest("1.0.0")
	bundle := buildFakeBundle(t, manifest, 1024)
	if err := Install(AvailablePack{Manifest: manifest, BundleURL: bundle.path, BundleSize: int64(len(bundle.bytes)), Local: true}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, installed := installedVersion(t); !installed {
		t.Error("local bundle is not installed")
	}
	if names := folderNames(t, filepath.Join(rootPath, downloadsFolderName)); len(names) != 0 {
		t.Errorf("local install used the downloads folder: %v", names)
	}
	if _, err := os.Stat(bundle.path); err != nil {
		t.Errorf("local bundle was removed: %v", err)
	}
}

func TestBundleWithADifferentManifestIsRejected(t *testing.T) {
	usePacksRoot(t)
	stubRunnerSteps(t, nil)

	bundle := buildFakeBundle(t, testManifest("2.0.0"), 1024)
	err := Install(AvailablePack{Manifest: testManifest("1.0.0"), BundleURL: bundle.path, Local: true})
	if err == nil || !strings.Contains(err.Error(), "not testpack 1.0.0") {
		t.Fatalf("Install error = %v, want a manifest mismatch", err)
	}
	if _, installed := installedVersion(t); installed {
		t.Error("mismatched bundle was installed")
	}
}

func TestUninstallRemovesEverything(t *testing.T) {
	rootPath := usePacksRoot(t)
	stubRunnerSteps(t, nil)
	installFromServer(t, "1.0.0", &bundleServer{})

	if err := Uninstall(testEngineID); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(filepath.Join(rootPath, testEngineID)); !os.IsNotExist(err) {
		t.Error("engine folder still exists")
	}
	if isRegistered(testEngineID) {
		t.Error("engine is still registered")
	}
}

func TestInvalidEngineIDsNeverBecomePaths(t *testing.T) {
	usePacksRoot(t)
	stubRunnerSteps(t, nil)

	manifest := testManifest("1.0.0")
	manifest.ID = `..\..\escape`
	if err := Install(AvailablePack{Manifest: manifest, BundleURL: "unused.nstudio", Local: true}); err == nil {
		t.Error("Install accepted a path-like engine id")
	}
	if err := Uninstall(`..\..`); err == nil {
		t.Error("Uninstall accepted a path-like engine id")
	}
}

func TestFetchAvailableReadsTheCatalog(t *testing.T) {
	usePacksRoot(t)
	compatible := testManifest("1.1.0")
	incompatible := testManifest("2.0.0")
	incompatible.Protocol = python.ProtocolVersion + 1
	otherPlatform := testManifest("1.1.0")
	otherPlatform.OS = "plan9"

	var catalogStatus atomic.Int32
	catalogStatus.Store(http.StatusOK)
	catalogServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if status := int(catalogStatus.Load()); status != http.StatusOK {
			writer.WriteHeader(status)
			return
		}
		json.NewEncoder(writer).Encode(catalogFile{SchemaVersion: 1, Packs: []catalogEntry{
			{Manifest: compatible, URL: "https://example.com/a.nstudio", Size: 10, SHA256: "aa", Notes: "New voices."},
			{Manifest: incompatible, URL: "https://example.com/b.nstudio", Size: 20},
			{Manifest: otherPlatform, URL: "https://example.com/c.nstudio", Size: 30},
		}})
	}))
	t.Cleanup(catalogServer.Close)
	previousCatalogURL := CatalogURL
	CatalogURL = catalogServer.URL
	t.Cleanup(func() { CatalogURL = previousCatalogURL })

	availablePacks, err := FetchAvailable()
	if err != nil {
		t.Fatalf("FetchAvailable: %v", err)
	}
	if len(availablePacks) != 2 {
		t.Fatalf("FetchAvailable returned %d packs, want the 2 for this platform", len(availablePacks))
	}
	if !availablePacks[0].Compatible || availablePacks[0].Notes != "New voices." || availablePacks[0].SHA256 != "aa" {
		t.Errorf("first pack = %+v", availablePacks[0])
	}
	if availablePacks[1].Compatible {
		t.Error("a pack with another protocol is marked compatible")
	}

	catalogStatus.Store(http.StatusNotFound)
	if availablePacks, err := FetchAvailable(); err != nil || len(availablePacks) != 0 {
		t.Errorf("missing catalog = %v, %v; want no packs and no error", availablePacks, err)
	}
}
