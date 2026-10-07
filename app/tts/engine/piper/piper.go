package piper

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"nstudio/app/common/audio"
	"nstudio/app/common/process"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/engine/piper/native"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

const (
	defaultSampleRate   = 22050
	stdoutQuietWindow   = 50 * time.Millisecond
	stdoutDrainDeadline = time.Second
)

// <editor-fold desc="Stream Router">
// streamRouter owns the piper process stdout for the lifetime of the process
// and mirrors it into the sink of the in-flight request; between requests
// output is discarded. Write never fails — piper's stdout must always keep
// draining or the subprocess stalls.
type streamRouter struct {
	mu        sync.Mutex
	sink      io.Writer
	lastWrite time.Time
}

func (router *streamRouter) Write(data []byte) (int, error) {
	router.mu.Lock()
	defer router.mu.Unlock()
	router.lastWrite = time.Now()
	if router.sink != nil {
		if _, err := router.sink.Write(data); err != nil {
			router.sink = nil
		}
	}
	return len(data), nil
}

func (router *streamRouter) Attach(sink io.Writer) {
	router.mu.Lock()
	defer router.mu.Unlock()
	router.sink = sink
	router.lastWrite = time.Now()
}

func (router *streamRouter) Detach() {
	router.mu.Lock()
	defer router.mu.Unlock()
	router.sink = nil
}

func (router *streamRouter) QuietSince() time.Duration {
	router.mu.Lock()
	defer router.mu.Unlock()
	return time.Since(router.lastWrite)
}

//</editor-fold>

// isNativeMode returns true when the engine should use the native DLL instead of the binary.
func (piper *Piper) isNativeMode() bool {
	return !config.GetEngine().Local.Piper.UseExecutable
}

// getNative lazily creates the native engine instance.
func (piper *Piper) getNative() *native.Piper {
	if piper.native == nil {
		piper.native = &native.Piper{}
	}
	return piper.native
}

// <editor-fold desc="Engine Interface">
func (piper *Piper) Initialize() error {
	if piper.isNativeMode() {
		return piper.getNative().Initialize()
	}

	settings := config.GetEngine().Local.Piper

	if settings.Location == "" {
		return response.Err("Piper executable location is not set")
	}

	if settings.ModelsDirectory == "" {
		return response.Err("Piper model location is not set")
	}

	if piper.models == nil {
		piper.models = make(map[string]*PiperInstance)
	}

	return nil
}

func (piper *Piper) Start(modelName string) error {
	if piper.isNativeMode() {
		return piper.getNative().Start(modelName)
	}

	var err error
	err = piper.Initialize()
	if err != nil {
		return response.Err("%w", err)
	}

	modelProcessID := piper.GetProcessID(modelName)
	if modelProcessID > 0 {
		response.Debug(util.MessageData{
			Summary: "Piper model '%s' already exists. " + modelName,
			Detail:  fmt.Sprintf("PID: %d", modelProcessID),
		})
		return nil
	}

	err, modelPath := util.ExpandPath(config.GetEngine().Local.Piper.ModelsDirectory)
	if err != nil {
		return response.Err("%w", err)
	}

	voices, err := readModelVoices(modelPath, modelName)
	if err != nil {
		return response.Err("%w", err)
	}

	onnxPath := filepath.Join(modelPath, modelName, fmt.Sprintf("%s.onnx", modelName))

	commandArguments := []string{"--model", onnxPath, "--json-input", "--output-raw"}

	err, piperPath := util.ExpandPath(config.GetEngine().Local.Piper.Location)
	if err != nil {
		return response.Err("%w", err)
	}

	command := exec.Command(piperPath, commandArguments...)

	process.HideCommandLine(command)

	response.Debug(util.MessageData{
		Summary: fmt.Sprintf("Preparing command: %s %s",
			command.Path,
			strings.Join(command.Args[1:], " "),
		),
	})

	instance := &PiperInstance{
		command:      command,
		capture:      &streamRouter{},
		sentinel:     make(chan struct{}, 1),
		stderrClosed: make(chan struct{}),
		sampleRate:   readModelSampleRate(onnxPath + ".json"),
		Voices:       voices,
	}

	instance.stdin, err = instance.command.StdinPipe()
	if err != nil {
		return response.Err("%w", err)
	}

	instance.stderr, err = instance.command.StderrPipe()
	if err != nil {
		return response.Err("%w", err)
	}

	instance.stdout, err = instance.command.StdoutPipe()
	if err != nil {
		return response.Err("%w", err)
	}

	err = instance.command.Start()
	if err != nil {
		return response.Err("%w", err)
	}

	piper.StartAudioCapture(instance)
	piper.watchCompletionSignals(instance)

	if piper.models == nil {
		piper.models = make(map[string]*PiperInstance)
	}

	piper.models[modelName] = instance

	return nil
}

// readModelSampleRate reads the output rate piper will synthesize at from the
// model's onnx config; models that ship without one keep the piper default.
func readModelSampleRate(configPath string) int {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return defaultSampleRate
	}

	var parsed struct {
		Audio struct {
			SampleRate int `json:"sample_rate"`
		} `json:"audio"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil || parsed.Audio.SampleRate <= 0 {
		return defaultSampleRate
	}

	return parsed.Audio.SampleRate
}

func (piper *Piper) Stop(modelName string) error {
	if piper.native != nil && piper.isNativeMode() {
		return piper.native.Stop(modelName)
	}

	defer delete(piper.models, modelName)

	instance, exists := piper.models[modelName]
	if !exists {
		response.Debug(util.MessageData{
			Summary: fmt.Sprintf("Instance for %s is not running", modelName),
		})
		return nil
	}

	if err := instance.command.Process.Signal(os.Interrupt); err != nil {
		if killErr := instance.command.Process.Kill(); killErr != nil {
			return response.Err("Failed to kill process for model %s: %v, original issue: %v", modelName, killErr, err)
		}
	}

	err := instance.command.Wait()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				// On Windows, killed processes exit with signal -1
				// On Unix, they may exit with SIGKILL (9) or SIGINT (2)
				// We accept these as successful terminations
				if status.Signaled() {
					signal := status.Signal()
					if signal == os.Interrupt || signal == os.Kill || signal == syscall.SIGTERM || signal == syscall.Signal(-1) {
						err = nil
					} else {
						return response.Err("Process for model %s exited with unexpected signal: %v", modelName, signal)
					}
				} else {
					// On Windows, killed processes often exit with status code 1
					exitCode := exitErr.ExitCode()
					if exitCode == 1 {
						err = nil
					}
				}
			}
		}

		if err != nil {
			return response.Err("Process for model %s exited with issue: %v", modelName, err)
		}
	}

	if instance.stdin != nil {
		if err := instance.stdin.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			return response.Err("Failed to close stdin for model %s: %v", modelName, err)
		}
	}
	if instance.stdout != nil {
		if err := instance.stdout.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			return response.Err("Failed to close stdout for model %s: %v", modelName, err)
		}
	}
	if instance.stderr != nil {
		if err := instance.stderr.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			return response.Err("Failed to close stderr for model %s: %v", modelName, err)
		}
	}

	response.Debug(util.MessageData{
		Summary: fmt.Sprintf("Stopped model: %s", modelName),
	})

	return nil
}

func (piper *Piper) Synthesize(ctx context.Context, request engine.SpeechRequest) (*audio.Stream, error) {
	if piper.isNativeMode() {
		return piper.getNative().Synthesize(ctx, request)
	}

	speakerID, _ := strconv.Atoi(request.Voice)
	payload, err := json.Marshal(PiperInputLite{
		Text:      request.Text,
		SpeakerID: speakerID,
	})
	if err != nil {
		return nil, response.Err("%w", err)
	}
	payload = append(payload, '\n')

	return piper.streamUtterance(request.Model, payload)
}

// streamUtterance sends one json-input line to the piper process and returns a
// live stream fed by its stdout. The instance mutex serializes requests on the
// one process; completion is piper's stderr " sec)" sentinel followed by a
// quiet-drain of stdout, because the sentinel can beat the last audio bytes
// through the pipe buffers.
func (piper *Piper) streamUtterance(model string, payload []byte) (*audio.Stream, error) {
	instance, err := piper.ensureRunning(model)
	if err != nil {
		return nil, err
	}

	if !utf8.Valid(payload) {
		return nil, response.Err("Input JSON is not valid UTF-8")
	}

	format := audio.StreamFormat{SampleRate: instance.sampleRate, Channels: 1, BitDepth: 16}
	return audio.NewProducedStream(format, func(writer *audio.StreamWriter) error {
		instance.mu.Lock()
		defer instance.mu.Unlock()

		select {
		case <-instance.sentinel:
		default:
		}

		response.Debug(util.MessageData{
			Summary: fmt.Sprintf("Sending to piper model: %s payload: %s", model, string(payload)),
		})

		instance.capture.Attach(writer)
		defer instance.capture.Detach()

		if _, err := instance.stdin.Write(payload); err != nil {
			return response.Err("%w", err)
		}

		select {
		case <-instance.sentinel:
		case <-instance.stderrClosed:
			return response.Err("piper process for model %s exited mid-synthesis", model)
		}

		drainDeadline := time.Now().Add(stdoutDrainDeadline)
		for instance.capture.QuietSince() < stdoutQuietWindow && time.Now().Before(drainDeadline) {
			time.Sleep(10 * time.Millisecond)
		}

		return nil
	}), nil
}

func (piper *Piper) ensureRunning(model string) (*PiperInstance, error) {
	if piper.GetProcessID(model) == 0 {
		if !config.GetEngineToggles()["piper"][model] {
			return nil, response.Err("Model is not enabled: %s", model)
		}

		response.NewWarn("Model is not running:" + model)

		if err := piper.Start(model); err != nil {
			return nil, response.Err("Failed to start model %s: %v", model, err)
		}
	}

	instance, exists := piper.models[model]
	if !exists {
		return nil, response.Err("Model %s is not running", model)
	}

	return instance, nil
}

func (piper *Piper) GetVoices(model string) ([]engine.Voice, error) {
	if piper.isNativeMode() {
		if piper.native != nil && piper.native.IsLoaded(model) {
			return piper.native.GetVoices(model)
		}
	} else if modelData, exists := piper.models[model]; exists {
		return modelData.Voices, nil
	}

	modelsPath, err := getModelsDir()
	if err != nil {
		return nil, response.Err("%w", err)
	}

	return readModelVoices(modelsPath, model)
}

func readModelVoices(modelsPath, modelName string) ([]engine.Voice, error) {
	data, err := os.ReadFile(filepath.Join(modelsPath, modelName, modelName+".metadata.json"))
	if err != nil {
		return nil, response.Err("read voices for piper model %s: %w", modelName, err)
	}

	var voices []engine.Voice
	if err := json.Unmarshal(data, &voices); err != nil {
		return nil, response.Err("parse voices for piper model %s: %w", modelName, err)
	}

	return voices, nil
}

func (piper *Piper) FetchModels() map[string]engine.Model {
	return FetchModels()
}

// </editor-fold>

// <editor-fold desc="Other">
func (piper *Piper) StartAudioCapture(instance *PiperInstance) {
	go func() {
		_, err := io.Copy(instance.capture, instance.stdout)
		if err != nil && !errors.Is(err, os.ErrClosed) {
			response.Err("Failed to start capturing audio: %s\n%w", instance.command.String(), err)
		}
	}()
}

func (piper *Piper) watchCompletionSignals(instance *PiperInstance) {
	go func() {
		scanner := bufio.NewScanner(instance.stderr)
		for scanner.Scan() {
			if strings.HasSuffix(scanner.Text(), " sec)") {
				select {
				case instance.sentinel <- struct{}{}:
				default:
				}
			}
		}
		close(instance.stderrClosed)
	}()
}

func (piper *Piper) GetProcessID(modelName string) int {
	instance, exists := piper.models[modelName]
	if !exists {
		return 0
	}

	if instance.command.Process == nil {
		return 0
	}

	if instance.command.ProcessState != nil && instance.command.ProcessState.Exited() {
		return 0
	}

	if !process.IsRunning(instance.command.Process) {
		return 0
	}

	return instance.command.Process.Pid
}

func FetchModels() map[string]engine.Model {
	return scanInstalledModels()
}

// scanInstalledModels walks the Piper models directory and returns every
// subfolder that contains the three required files as an engine.Model entry.
func scanInstalledModels() map[string]engine.Model {
	result := make(map[string]engine.Model)

	modelsDir, err := getModelsDir()
	if err != nil || modelsDir == "" {
		return result
	}

	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		if !isModelInstalled(modelsDir, id) {
			continue
		}

		result[id] = engine.Model{
			ID:     id,
			Name:   id,
			Engine: "piper",
		}
	}

	return result
}

// </editor-fold>
