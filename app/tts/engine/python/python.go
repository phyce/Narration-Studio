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
	interpreterPath := filepath.Join(pythonEngine.BundlePath, filepath.FromSlash(pythonEngine.Manifest.Runtime.Python))
	if _, err := os.Stat(interpreterPath); err != nil {
		return response.Err("bundle missing python interpreter at %s: %w", interpreterPath, err)
	}
	entrypointPath := filepath.Join(pythonEngine.BundlePath, filepath.FromSlash(pythonEngine.Manifest.Runtime.Entrypoint))
	if _, err := os.Stat(entrypointPath); err != nil {
		return response.Err("bundle missing runner entrypoint at %s: %w", entrypointPath, err)
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
		return loadedRunner.streamSynth(ctx, synthRequest, writer)
	}), nil
}

func (pythonEngine *Python) GetVoices(model string) ([]engine.Voice, error) {
	var modelFound bool
	var voices []engine.Voice
	for index := range pythonEngine.Manifest.Models {
		if pythonEngine.Manifest.Models[index].ID == model {
			voices = append(voices, pythonEngine.Manifest.Models[index].ToVoices()...)
			modelFound = true
			break
		}
	}
	if !modelFound {
		return nil, response.Err("python: model %q not found in manifest %q", model, pythonEngine.Manifest.ID)
	}

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
		"Loading %s model %q + encoding %d voice reference(s) — first run also downloads weights from HuggingFace",
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

func childProcessEnvironment(bundlePath string, extraEnvironmentVariables map[string]string) []string {
	sitePackagesPath := filepath.Join(bundlePath, "site-packages")

	parentEnvironment := os.Environ()
	mergedEnvironment := make([]string, 0, len(parentEnvironment)+len(extraEnvironmentVariables)+2)

	for _, environmentEntry := range parentEnvironment {
		name, _, _ := strings.Cut(environmentEntry, "=")
		if _, overridden := extraEnvironmentVariables[name]; overridden {
			continue
		}
		if name == "PYTHONPATH" {
			continue
		}
		mergedEnvironment = append(mergedEnvironment, environmentEntry)
	}

	pythonPath := sitePackagesPath
	if existingPythonPath, ok := os.LookupEnv("PYTHONPATH"); ok && existingPythonPath != "" {
		pythonPath = sitePackagesPath + string(os.PathListSeparator) + existingPythonPath
	}
	mergedEnvironment = append(mergedEnvironment, "PYTHONPATH="+pythonPath)
	mergedEnvironment = append(mergedEnvironment, "PYTHONUNBUFFERED=1")
	if runtime.GOOS == "windows" {
		mergedEnvironment = append(mergedEnvironment, "PYTHONIOENCODING=utf-8")
	}

	for name, value := range extraEnvironmentVariables {
		mergedEnvironment = append(mergedEnvironment, name+"="+value)
	}
	return mergedEnvironment
}
