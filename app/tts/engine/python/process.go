package python

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"nstudio/app/common/audio"
	"nstudio/app/common/process"
	"nstudio/app/common/response"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/log"
)

const (
	defaultBootTimeoutSec = 60
)

var (
	ErrRunnerExited  = errors.New("python runner exited")
	ErrRunnerTimeout = errors.New("python runner timed out")
)

// runner drives the bundle's runner (runner.py, or its own program) over the streaming stdio protocol
// (docs/python-engine-protocol.md): stdin carries JSON-line requests, stdout
// is a raw s16le PCM firehose routed straight into the active stream writer,
// stderr carries JSON event lines (ready/done/error) interleaved with
// free-form log lines.
type runner struct {
	engineID   string
	bundlePath string

	command *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser

	writeMutex sync.Mutex

	format audio.StreamFormat

	sinkMutex   sync.Mutex
	sink        *audio.StreamWriter
	routedBytes int64

	pendingMutex     sync.Mutex
	pending          map[string]chan runnerEvent
	progressHandlers map[string]func(runnerEvent)

	nextRequestID atomic.Uint64
	exited        atomic.Bool
	exitError     atomic.Value

	processDone chan struct{}
}

type runnerEvent struct {
	ID           string `json:"id,omitempty"`
	Event        string `json:"event"`
	Runner       string `json:"runner,omitempty"`
	Version      string `json:"version,omitempty"`
	Error        string `json:"error,omitempty"`
	Code         string `json:"code,omitempty"`
	SampleRate   int    `json:"sampleRate,omitempty"`
	Channels     int    `json:"channels,omitempty"`
	Bits         int    `json:"bits,omitempty"`
	TotalSamples int    `json:"totalSamples,omitempty"`
	Downloaded   int64  `json:"downloaded,omitempty"`
	Total        int64  `json:"total,omitempty"`
}

func newRunner(engineID, bundlePath string, manifest *Manifest) (*runner, error) {
	pythonRunner := &runner{
		engineID:         engineID,
		bundlePath:       bundlePath,
		pending:          make(map[string]chan runnerEvent),
		progressHandlers: make(map[string]func(runnerEvent)),
		processDone:      make(chan struct{}),
	}

	command := runnerCommand(bundlePath, manifest.Runtime)
	process.HideCommandLine(command)

	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, response.Err("stdin pipe: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, response.Err("stdout pipe: %w", err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return nil, response.Err("stderr pipe: %w", err)
	}

	if err := command.Start(); err != nil {
		return nil, response.Err("start %s: %w", command.Path, err)
	}

	pythonRunner.command = command
	pythonRunner.stdin = stdin
	pythonRunner.stdout = stdout
	pythonRunner.stderr = stderr

	go pythonRunner.routeStdout()
	go pythonRunner.readEvents()
	go pythonRunner.watchExit()

	bootTimeout := time.Duration(manifest.Runtime.BootTimeoutSec) * time.Second
	if bootTimeout <= 0 {
		bootTimeout = defaultBootTimeoutSec * time.Second
	}
	if err := pythonRunner.waitForReady(bootTimeout); err != nil {
		_ = pythonRunner.kill()
		return nil, err
	}

	return pythonRunner, nil
}

func (pythonRunner *runner) waitForReady(bootTimeout time.Duration) error {
	readyChannel := pythonRunner.registerPending("__ready__")
	defer pythonRunner.cancelPending("__ready__")

	select {
	case readyEvent, ok := <-readyChannel:
		if !ok {
			return pythonRunner.exitFailure()
		}
		if readyEvent.Event == "error" {
			return response.Err("runner failed to start: %s", readyEvent.Error)
		}
		return nil
	case <-pythonRunner.processDone:
		return pythonRunner.exitFailure()
	case <-time.After(bootTimeout):
		return response.Err("runner boot timed out after %s", bootTimeout)
	}
}

func (pythonRunner *runner) exitFailure() error {
	if err, _ := pythonRunner.exitError.Load().(error); err != nil {
		return response.Err("%w: %s", ErrRunnerExited, err.Error())
	}
	return response.Err("%w", ErrRunnerExited)
}

// routeStdout owns stdout for the life of the process. PCM bytes are mirrored
// into the active sink (or discarded between requests); routedBytes always
// counts the current request's bytes so completion can be detected even after
// the consumer abandoned the stream.
func (pythonRunner *runner) routeStdout() {
	chunk := make([]byte, 32*1024)
	for {
		count, err := pythonRunner.stdout.Read(chunk)
		if count > 0 {
			pythonRunner.sinkMutex.Lock()
			pythonRunner.routedBytes += int64(count)
			if pythonRunner.sink != nil {
				if _, writeErr := pythonRunner.sink.Write(chunk[:count]); writeErr != nil {
					pythonRunner.sink = nil
				}
			}
			pythonRunner.sinkMutex.Unlock()
		}
		if err != nil {
			return
		}
	}
}

// readEvents parses stderr: JSON objects carrying an "event" field are
// protocol events; anything else is treated as a runner log line.
func (pythonRunner *runner) readEvents() {
	scanner := bufio.NewScanner(pythonRunner.stderr)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		if trimmed[0] == '{' {
			var event runnerEvent
			if err := json.Unmarshal(trimmed, &event); err == nil && event.Event != "" {
				pythonRunner.dispatch(event)
				continue
			}
		}
		log.Infof("[python:%s] %s", pythonRunner.engineID, string(trimmed))
	}
}

func (pythonRunner *runner) dispatch(event runnerEvent) {
	eventID := event.ID
	switch event.Event {
	case "ready":
		eventID = "__ready__"
	case "chunk":
		return
	case "progress":
		pythonRunner.pendingMutex.Lock()
		handleProgress := pythonRunner.progressHandlers[eventID]
		pythonRunner.pendingMutex.Unlock()
		if handleProgress != nil {
			handleProgress(event)
		}
		return
	case "done", "error":
	default:
		log.Warnf("[python:%s] unknown event %q: %+v", pythonRunner.engineID, event.Event, event)
		return
	}
	if eventID == "" {
		log.Warnf("[python:%s] event with no id: %+v", pythonRunner.engineID, event)
		return
	}

	pythonRunner.pendingMutex.Lock()
	pendingChannel, exists := pythonRunner.pending[eventID]
	if exists {
		delete(pythonRunner.pending, eventID)
	}
	pythonRunner.pendingMutex.Unlock()

	if !exists {
		log.Warnf("[python:%s] event for unknown id %q: %+v", pythonRunner.engineID, eventID, event)
		return
	}
	select {
	case pendingChannel <- event:
	default:
	}
}

func (pythonRunner *runner) watchExit() {
	err := pythonRunner.command.Wait()
	pythonRunner.exited.Store(true)
	if err != nil {
		pythonRunner.exitError.Store(err)
	}
	pythonRunner.pendingMutex.Lock()
	for eventID, pendingChannel := range pythonRunner.pending {
		close(pendingChannel)
		delete(pythonRunner.pending, eventID)
	}
	pythonRunner.pendingMutex.Unlock()
	close(pythonRunner.processDone)
}

func (pythonRunner *runner) registerPending(eventID string) chan runnerEvent {
	pythonRunner.pendingMutex.Lock()
	defer pythonRunner.pendingMutex.Unlock()
	pendingChannel, exists := pythonRunner.pending[eventID]
	if !exists {
		pendingChannel = make(chan runnerEvent, 1)
		pythonRunner.pending[eventID] = pendingChannel
	}
	return pendingChannel
}

func (pythonRunner *runner) cancelPending(eventID string) {
	pythonRunner.pendingMutex.Lock()
	if pendingChannel, exists := pythonRunner.pending[eventID]; exists {
		delete(pythonRunner.pending, eventID)
		close(pendingChannel)
	}
	pythonRunner.pendingMutex.Unlock()
}

func (pythonRunner *runner) writeRequest(request runnerRequest) error {
	requestPayload, err := json.Marshal(request)
	if err != nil {
		return response.Err("marshal request: %w", err)
	}
	requestPayload = append(requestPayload, '\n')

	pythonRunner.writeMutex.Lock()
	_, writeErr := pythonRunner.stdin.Write(requestPayload)
	pythonRunner.writeMutex.Unlock()
	if writeErr != nil {
		return response.Err("stdin write: %w", writeErr)
	}
	return nil
}

// request performs a non-streaming op (load/unload) and returns its terminal
// event.
func (pythonRunner *runner) request(request runnerRequest, timeout time.Duration) (runnerEvent, error) {
	if pythonRunner.exited.Load() {
		return runnerEvent{}, pythonRunner.exitFailure()
	}

	if request.ID == "" {
		request.ID = fmt.Sprintf("r%d", pythonRunner.nextRequestID.Add(1))
	}
	eventChannel := pythonRunner.registerPending(request.ID)
	defer pythonRunner.cancelPending(request.ID)

	if err := pythonRunner.writeRequest(request); err != nil {
		return runnerEvent{}, err
	}

	select {
	case event, ok := <-eventChannel:
		if !ok {
			return runnerEvent{}, pythonRunner.exitFailure()
		}
		return event, nil
	case <-pythonRunner.processDone:
		return runnerEvent{}, pythonRunner.exitFailure()
	case <-time.After(timeout):
		return runnerEvent{}, ErrRunnerTimeout
	}
}

func (pythonRunner *runner) load(model string, voicePaths []string) error {
	event, err := pythonRunner.request(runnerRequest{Op: "load", Model: model, Voices: voicePaths}, time.Duration(defaultGenerateTimeoutSec)*time.Second)
	if err != nil {
		return response.Err("%w", err)
	}
	if event.Event == "error" {
		return response.Err("%s", event.Error)
	}
	if event.SampleRate <= 0 {
		return response.Err("load response is missing sampleRate")
	}
	channels := event.Channels
	if channels <= 0 {
		channels = 1
	}
	bits := event.Bits
	if bits <= 0 {
		bits = 16
	}
	pythonRunner.format = audio.StreamFormat{SampleRate: event.SampleRate, Channels: channels, BitDepth: bits}
	return nil
}

func (pythonRunner *runner) prepare(reportProgress func(downloaded, total int64), stallTimeout time.Duration) error {
	if pythonRunner.exited.Load() {
		return pythonRunner.exitFailure()
	}

	requestID := fmt.Sprintf("r%d", pythonRunner.nextRequestID.Add(1))
	eventChannel := pythonRunner.registerPending(requestID)
	defer pythonRunner.cancelPending(requestID)

	progressArrived := make(chan struct{}, 1)
	pythonRunner.setProgressHandler(requestID, func(event runnerEvent) {
		reportProgress(event.Downloaded, event.Total)
		select {
		case progressArrived <- struct{}{}:
		default:
		}
	})
	defer pythonRunner.setProgressHandler(requestID, nil)

	if err := pythonRunner.writeRequest(runnerRequest{ID: requestID, Op: "prepare"}); err != nil {
		return err
	}

	stallTimer := time.NewTimer(stallTimeout)
	defer stallTimer.Stop()

	for {
		select {
		case event, ok := <-eventChannel:
			if !ok {
				return pythonRunner.exitFailure()
			}
			if event.Event == "error" {
				return response.Err("%s", event.Error)
			}
			return nil
		case <-progressArrived:
			stallTimer.Reset(stallTimeout)
		case <-pythonRunner.processDone:
			return pythonRunner.exitFailure()
		case <-stallTimer.C:
			return response.Err("no download progress for %s: %w", stallTimeout, ErrRunnerTimeout)
		}
	}
}

func (pythonRunner *runner) setProgressHandler(requestID string, handleProgress func(runnerEvent)) {
	pythonRunner.pendingMutex.Lock()
	defer pythonRunner.pendingMutex.Unlock()
	if handleProgress == nil {
		delete(pythonRunner.progressHandlers, requestID)
		return
	}
	pythonRunner.progressHandlers[requestID] = handleProgress
}

func (pythonRunner *runner) streamSynth(ctx context.Context, request runnerRequest, format audio.StreamFormat, writer *audio.StreamWriter) error {
	if pythonRunner.exited.Load() {
		return pythonRunner.exitFailure()
	}

	request.ID = fmt.Sprintf("r%d", pythonRunner.nextRequestID.Add(1))
	eventChannel := pythonRunner.registerPending(request.ID)
	defer pythonRunner.cancelPending(request.ID)

	pythonRunner.attachSink(writer)
	defer pythonRunner.detachSink()

	if err := pythonRunner.writeRequest(request); err != nil {
		return err
	}

	generateTimeout := time.NewTimer(time.Duration(defaultGenerateTimeoutSec) * time.Second)
	defer generateTimeout.Stop()

	var terminalEvent runnerEvent
	terminalReceived := false
	cancelled := false

	select {
	case event, ok := <-eventChannel:
		if !ok {
			return pythonRunner.exitFailure()
		}
		terminalEvent = event
		terminalReceived = true
	case <-ctx.Done():
		cancelled = true
		pythonRunner.detachSink()
		select {
		case event, ok := <-eventChannel:
			if ok {
				terminalEvent = event
				terminalReceived = true
			}
		case <-pythonRunner.processDone:
		case <-generateTimeout.C:
		}
	case <-pythonRunner.processDone:
		return pythonRunner.exitFailure()
	case <-generateTimeout.C:
		return response.Err("python synth: %w", ErrRunnerTimeout)
	}

	if terminalReceived && terminalEvent.Event == "error" {
		pythonRunner.drainQuiet()
		return response.Err("python synth: %s", terminalEvent.Error)
	}
	if terminalReceived {
		expectedBytes := int64(terminalEvent.TotalSamples) * 2
		pythonRunner.awaitRoutedBytes(expectedBytes)
		if terminalEvent.SampleRate > 0 && terminalEvent.SampleRate != format.SampleRate {
			log.Warnf("[python:%s] synth sample rate %d differs from the voice's format %d",
				pythonRunner.engineID, terminalEvent.SampleRate, format.SampleRate)
		}
	}

	if cancelled {
		return ctx.Err()
	}
	return nil
}

func (pythonRunner *runner) attachSink(writer *audio.StreamWriter) {
	pythonRunner.sinkMutex.Lock()
	pythonRunner.sink = writer
	pythonRunner.routedBytes = 0
	pythonRunner.sinkMutex.Unlock()
}

func (pythonRunner *runner) detachSink() {
	pythonRunner.sinkMutex.Lock()
	pythonRunner.sink = nil
	pythonRunner.sinkMutex.Unlock()
}

func (pythonRunner *runner) currentRoutedBytes() int64 {
	pythonRunner.sinkMutex.Lock()
	defer pythonRunner.sinkMutex.Unlock()
	return pythonRunner.routedBytes
}

// awaitRoutedBytes waits until the request's full PCM payload has cleared the
// stdout pipe. The runner emits done only after writing everything, so this is
// a pipe-flush wait, not a generation wait.
func (pythonRunner *runner) awaitRoutedBytes(expectedBytes int64) {
	if expectedBytes <= 0 {
		pythonRunner.drainQuiet()
		return
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if pythonRunner.currentRoutedBytes() >= expectedBytes || pythonRunner.exited.Load() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	log.Warnf("[python:%s] stdout drain timed out: %d of %d bytes",
		pythonRunner.engineID, pythonRunner.currentRoutedBytes(), expectedBytes)
}

func (pythonRunner *runner) drainQuiet() {
	quietDeadline := time.Now().Add(time.Second)
	lastCount := pythonRunner.currentRoutedBytes()
	lastGrowth := time.Now()
	for time.Now().Before(quietDeadline) {
		time.Sleep(5 * time.Millisecond)
		count := pythonRunner.currentRoutedBytes()
		if count != lastCount {
			lastCount = count
			lastGrowth = time.Now()
			continue
		}
		if time.Since(lastGrowth) >= 50*time.Millisecond {
			return
		}
	}
}

func (pythonRunner *runner) close(shutdownGrace time.Duration) error {
	if pythonRunner.exited.Load() {
		return nil
	}

	_ = pythonRunner.writeRequest(runnerRequest{ID: fmt.Sprintf("r%d", pythonRunner.nextRequestID.Add(1)), Op: "shutdown"})
	_ = pythonRunner.stdin.Close()

	select {
	case <-pythonRunner.processDone:
		return nil
	case <-time.After(shutdownGrace):
		return pythonRunner.kill()
	}
}

func (pythonRunner *runner) kill() error {
	if pythonRunner.command.Process == nil {
		return nil
	}
	if err := pythonRunner.command.Process.Kill(); err != nil {
		return response.Err("kill runner: %w", err)
	}
	return nil
}
