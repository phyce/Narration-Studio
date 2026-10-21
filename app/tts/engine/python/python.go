package python

import (
	"context"
	"fmt"
	"nstudio/app/common/audio"
	"nstudio/app/common/response"
	"nstudio/app/common/status"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/engine/python/voicepack"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	defaultGenerateTimeoutSec = 300
	defaultShutdownGraceSec   = 5
	maxRestartsPerWindow      = 3
	restartWindow             = time.Minute
)

const ProtocolVersion = 1

func SupportsProtocol(protocol int) bool {
	return protocol == ProtocolVersion
}

var prepareStallTimeout = 2 * time.Minute

func PrepareBundle(ctx context.Context, manifest Manifest, bundlePath string, reportProgress func(downloaded, total int64)) error {
	preparingRunner, err := newRunner(manifest.ID, bundlePath, &manifest)
	if err != nil {
		return response.Err("start runner: %w", err)
	}
	defer preparingRunner.close(time.Duration(defaultShutdownGraceSec) * time.Second)
	stopKillingOnCancel := context.AfterFunc(ctx, func() { _ = preparingRunner.kill() })
	defer stopKillingOnCancel()

	if err := preparingRunner.prepare(reportProgress, prepareStallTimeout); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	return nil
}

type Python struct {
	Manifest   Manifest
	BundlePath string

	mutex        sync.Mutex
	runner       *runner
	currentModel string

	restartMutex   sync.Mutex
	recentRestarts []time.Time
}

func New(manifest Manifest, bundlePath string) *Python {
	return &Python{
		Manifest:   manifest,
		BundlePath: bundlePath,
	}
}

func (pythonEngine *Python) Initialize() error {
	if pythonEngine.BundlePath == "" {
		return response.Err("python engine has no bundle path")
	}
	for _, runnerFile := range pythonEngine.Manifest.Runtime.RunnerFiles() {
		runnerFilePath := bundleFilePath(pythonEngine.BundlePath, runnerFile)
		if _, err := os.Stat(runnerFilePath); err != nil {
			return response.Err("bundle missing its runner file %s: %w", runnerFilePath, err)
		}
	}
	return nil
}

func (pythonEngine *Python) Start(modelName string) error {
	pythonEngine.mutex.Lock()
	defer pythonEngine.mutex.Unlock()
	return pythonEngine.ensureModelLoaded(modelName)
}

func (pythonEngine *Python) Stop(modelName string) error {
	pythonEngine.mutex.Lock()
	defer pythonEngine.mutex.Unlock()

	if pythonEngine.runner == nil {
		return nil
	}
	if modelName != "" && pythonEngine.currentModel != "" && pythonEngine.currentModel != modelName {
		return nil
	}

	err := pythonEngine.runner.close(time.Duration(defaultShutdownGraceSec) * time.Second)
	pythonEngine.runner = nil
	pythonEngine.currentModel = ""
	return err
}

func (pythonEngine *Python) Synthesize(ctx context.Context, request engine.SpeechRequest) (*audio.Stream, error) {
	pythonEngine.mutex.Lock()
	if err := pythonEngine.ensureModelLoaded(request.Model); err != nil {
		pythonEngine.mutex.Unlock()
		return nil, response.Err("%w", err)
	}

	loadedRunner := pythonEngine.runner
	format := loadedRunner.format
	if manifestModel, found := pythonEngine.Manifest.findModel(request.Model); found {
		format = manifestModel.VoiceFormat(request.Voice, format)
	}

	voice := request.Voice
	if resolvedVoicePath, found := voicepack.ResolvePath(pythonEngine.Manifest.ID, voice); found {
		voice = resolvedVoicePath
	}
	pythonEngine.mutex.Unlock()

	synthRequest := runnerRequest{
		Op:     "synth",
		Model:  request.Model,
		Voice:  voice,
		Text:   request.Text,
		Params: request.Params,
	}

	return audio.NewProducedStream(format, func(writer *audio.StreamWriter) error {
		pythonEngine.mutex.Lock()
		defer pythonEngine.mutex.Unlock()
		if pythonEngine.runner != loadedRunner {
			return response.Err("python: runner restarted before synthesis started")
		}
		return loadedRunner.streamSynth(ctx, synthRequest, format, writer)
	}), nil
}

func (pythonEngine *Python) GetVoices(model string) ([]engine.Voice, error) {
	manifestModel, found := pythonEngine.Manifest.findModel(model)
	if !found {
		return nil, response.Err("python: model %q not found in manifest %q", model, pythonEngine.Manifest.ID)
	}
	voices := manifestModel.ToVoices()

	if packVoices, err := voicepack.AllVoices(pythonEngine.Manifest.ID); err == nil {
		voices = append(voices, packVoices...)
	}
	return voices, nil
}

func (pythonEngine *Python) FetchModels() map[string]engine.Model {
	return modelsFromManifest(pythonEngine.Manifest)
}

func (pythonEngine *Python) ensureModelLoaded(model string) error {
	if model == "" {
		return response.Err("python: model name is empty")
	}

	if pythonEngine.runner != nil && pythonEngine.runner.exited.Load() {
		_ = pythonEngine.runner.close(time.Second)
		pythonEngine.runner = nil
		pythonEngine.currentModel = ""
	}

	if pythonEngine.runner != nil && pythonEngine.currentModel == model {
		return nil
	}

	if pythonEngine.runner != nil && pythonEngine.currentModel != model {
		if err := pythonEngine.runner.close(time.Duration(defaultShutdownGraceSec) * time.Second); err != nil {
			response.Err("%w", err)
		}
		pythonEngine.runner = nil
		pythonEngine.currentModel = ""
	}

	if !pythonEngine.allowRestart() {
		return response.Err("python: runner has crashed %d times in the last %s; refusing to restart (disable and re-enable the engine to retry)", maxRestartsPerWindow, restartWindow)
	}

	status.Set(status.Loading, fmt.Sprintf("Starting %s engine...", pythonEngine.Manifest.Name))
	newlyStartedRunner, err := newRunner(pythonEngine.Manifest.ID, pythonEngine.BundlePath, &pythonEngine.Manifest)
	if err != nil {
		status.Set(status.Ready, "")
		return response.Err("python: spawn runner: %w", err)
	}

	installedVoicePaths := pythonEngine.allInstalledVoicePaths()

	status.Set(status.Loading, fmt.Sprintf(
		"Loading %s model %q + encoding %d voice reference(s)",
		pythonEngine.Manifest.Name, model, len(installedVoicePaths),
	))
	if err := newlyStartedRunner.load(model, installedVoicePaths); err != nil {
		_ = newlyStartedRunner.close(time.Second)
		status.Set(status.Ready, "")
		return response.Err("python: load model %q: %w", model, err)
	}

	pythonEngine.runner = newlyStartedRunner
	pythonEngine.currentModel = model
	status.Set(status.Ready, fmt.Sprintf("%s ready", pythonEngine.Manifest.Name))
	return nil
}

func (pythonEngine *Python) allInstalledVoicePaths() []string {
	packs, err := voicepack.ScanInstalled(pythonEngine.Manifest.ID)
	if err != nil {
		return nil
	}
	var installedVoicePaths []string
	for _, voicePack := range packs {
		for _, voice := range voicePack.Voices {
			absoluteVoicePath := filepath.Join(voicePack.PackPath, filepath.FromSlash(voice.File))
			if _, err := os.Stat(absoluteVoicePath); err == nil {
				installedVoicePaths = append(installedVoicePaths, absoluteVoicePath)
			}
		}
	}
	return installedVoicePaths
}

func (pythonEngine *Python) allowRestart() bool {
	pythonEngine.restartMutex.Lock()
	defer pythonEngine.restartMutex.Unlock()

	now := time.Now()
	cutoffTime := now.Add(-restartWindow)
	keptRestarts := pythonEngine.recentRestarts[:0]
	for _, restartTime := range pythonEngine.recentRestarts {
		if restartTime.After(cutoffTime) {
			keptRestarts = append(keptRestarts, restartTime)
		}
	}
	pythonEngine.recentRestarts = keptRestarts

	if len(pythonEngine.recentRestarts) >= maxRestartsPerWindow {
		return false
	}
	pythonEngine.recentRestarts = append(pythonEngine.recentRestarts, now)
	return true
}

func bundleFilePath(bundlePath, bundleRelativePath string) string {
	return filepath.Join(bundlePath, filepath.FromSlash(bundleRelativePath))
}

func runnerCommand(bundlePath string, packRuntime Runtime) *exec.Cmd {
	arguments := []string{"--bundle-dir", bundlePath}
	if packRuntime.RunsPython() {
		arguments = append([]string{bundleFilePath(bundlePath, packRuntime.Entrypoint)}, arguments...)
	}
	command := exec.Command(bundleFilePath(bundlePath, packRuntime.Program()), arguments...)
	command.Dir = bundlePath
	command.Env = childProcessEnvironment(bundlePath, packRuntime)
	return command
}

func childProcessEnvironment(bundlePath string, packRuntime Runtime) []string {
	extraEnvironmentVariables := packRuntime.Env
	parentEnvironment := os.Environ()
	mergedEnvironment := make([]string, 0, len(parentEnvironment)+len(extraEnvironmentVariables)+3)

	for _, environmentEntry := range parentEnvironment {
		name, _, _ := strings.Cut(environmentEntry, "=")
		if _, overridden := extraEnvironmentVariables[name]; overridden {
			continue
		}
		if packRuntime.RunsPython() && name == "PYTHONPATH" {
			continue
		}
		mergedEnvironment = append(mergedEnvironment, environmentEntry)
	}

	if packRuntime.RunsPython() {
		mergedEnvironment = append(mergedEnvironment, pythonEnvironment(bundlePath)...)
	}

	for name, value := range extraEnvironmentVariables {
		mergedEnvironment = append(mergedEnvironment, name+"="+value)
	}
	return mergedEnvironment
}

func pythonEnvironment(bundlePath string) []string {
	sitePackagesPath := filepath.Join(bundlePath, "site-packages")
	pythonPath := sitePackagesPath
	if existingPythonPath, ok := os.LookupEnv("PYTHONPATH"); ok && existingPythonPath != "" {
		pythonPath = sitePackagesPath + string(os.PathListSeparator) + existingPythonPath
	}
	environment := []string{"PYTHONPATH=" + pythonPath, "PYTHONUNBUFFERED=1"}
	if runtime.GOOS == "windows" {
		environment = append(environment, "PYTHONIOENCODING=utf-8")
	}
	return environment
}
