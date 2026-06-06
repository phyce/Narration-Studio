package python

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"nstudio/app/common/process"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/log"
)

const (
	defaultBootTimeoutSec = 60
)

type runner struct {
	engineID   string
	bundlePath string
	manifest   *Manifest

	mutex        sync.Mutex
	command      *exec.Cmd
	stdin        io.WriteCloser
	stdoutReader *bufio.Reader
	stderrReader io.ReadCloser

	pendingMutex sync.Mutex
	pending      map[string]chan rpcResponse

	nextRequestID atomic.Uint64
	exited        atomic.Bool
	exitError     atomic.Value

	readerDone chan struct{}
}

var (
	ErrRunnerExited  = errors.New("python runner exited")
	ErrRunnerTimeout = errors.New("python runner timed out")
)

func newRunner(engineID, bundlePath string, manifest *Manifest) (*runner, error) {
	pythonRunner := &runner{
		engineID:   engineID,
		bundlePath: bundlePath,
		manifest:   manifest,
		pending:    make(map[string]chan rpcResponse),
		readerDone: make(chan struct{}),
	}

	interpreterPath := filepath.Join(bundlePath, filepath.FromSlash(manifest.Runtime.Python))
	entrypointPath := filepath.Join(bundlePath, filepath.FromSlash(manifest.Runtime.Entrypoint))

	command := exec.Command(interpreterPath, entrypointPath, "--bundle-dir", bundlePath)
	command.Dir = bundlePath
	command.Env = childProcessEnvironment(bundlePath, manifest.Runtime.Env)

	process.HideCommandLine(command)

	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", interpreterPath, err)
	}

	pythonRunner.command = command
	pythonRunner.stdin = stdin
	pythonRunner.stdoutReader = bufio.NewReaderSize(stdout, 1<<16)
	pythonRunner.stderrReader = stderr

	go pythonRunner.readStderr()
	go pythonRunner.readStdout()
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
	select {
	case readyEvent := <-pythonRunner.readyChannel():
		if !readyEvent.Ok && readyEvent.Error != "" {
			return fmt.Errorf("runner failed to start: %s", readyEvent.Error)
		}
		return nil
	case <-pythonRunner.readerDone:
		if err, _ := pythonRunner.exitError.Load().(error); err != nil {
			return err
		}
		return ErrRunnerExited
	case <-time.After(bootTimeout):
		return fmt.Errorf("runner boot timed out after %s", bootTimeout)
	}
}

func (pythonRunner *runner) readyChannel() chan rpcResponse {
	pythonRunner.pendingMutex.Lock()
	defer pythonRunner.pendingMutex.Unlock()
	readyResponseChannel, alreadyRegistered := pythonRunner.pending["__ready__"]
	if !alreadyRegistered {
		readyResponseChannel = make(chan rpcResponse, 1)
		pythonRunner.pending["__ready__"] = readyResponseChannel
	}
	return readyResponseChannel
}

func (pythonRunner *runner) readStdout() {
	defer close(pythonRunner.readerDone)
	for {
		line, err := pythonRunner.stdoutReader.ReadBytes('\n')
		if len(line) > 0 {
			var responseMessage rpcResponse
			if unmarshalErr := json.Unmarshal(line, &responseMessage); unmarshalErr != nil {
				log.Warnf("[python:%s] non-JSON stdout line: %s", pythonRunner.engineID, string(line))
				continue
			}
			pythonRunner.dispatch(responseMessage)
		}
		if err != nil {
			if err != io.EOF {
				pythonRunner.exitError.Store(fmt.Errorf("stdout read error: %w", err))
			}
			return
		}
	}
}

func (pythonRunner *runner) readStderr() {
	scanner := bufio.NewScanner(pythonRunner.stderrReader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		log.Infof("[python:%s] %s", pythonRunner.engineID, scanner.Text())
	}
}

func (pythonRunner *runner) watchExit() {
	err := pythonRunner.command.Wait()
	pythonRunner.exited.Store(true)
	if err != nil {
		pythonRunner.exitError.Store(err)
	}
	pythonRunner.pendingMutex.Lock()
	for requestID, pendingChannel := range pythonRunner.pending {
		failure := ErrRunnerExited
		if err != nil {
			failure = fmt.Errorf("%w: %s", ErrRunnerExited, err.Error())
		}
		select {
		case pendingChannel <- rpcResponse{ID: requestID, Ok: false, Error: failure.Error()}:
		default:
		}
		close(pendingChannel)
		delete(pythonRunner.pending, requestID)
	}
	pythonRunner.pendingMutex.Unlock()
}

func (pythonRunner *runner) dispatch(responseMessage rpcResponse) {
	responseID := responseMessage.ID
	if responseMessage.Event == "ready" {
		responseID = "__ready__"
		responseMessage.Ok = true
	}
	if responseID == "" {
		log.Warnf("[python:%s] response with no id: %+v", pythonRunner.engineID, responseMessage)
		return
	}
	pythonRunner.pendingMutex.Lock()
	pendingChannel, exists := pythonRunner.pending[responseID]
	if exists {
		delete(pythonRunner.pending, responseID)
	}
	pythonRunner.pendingMutex.Unlock()

	if !exists {
		log.Warnf("[python:%s] response for unknown id %q: %+v", pythonRunner.engineID, responseID, responseMessage)
		return
	}
	select {
	case pendingChannel <- responseMessage:
	default:
		log.Warnf("[python:%s] response channel for id %q was full; dropping", pythonRunner.engineID, responseID)
	}
}

func (pythonRunner *runner) send(request rpcRequest, timeout time.Duration) (rpcResponse, error) {
	if pythonRunner.exited.Load() {
		if err, _ := pythonRunner.exitError.Load().(error); err != nil {
			return rpcResponse{}, fmt.Errorf("%w: %s", ErrRunnerExited, err.Error())
		}
		return rpcResponse{}, ErrRunnerExited
	}

	if request.ID == "" {
		request.ID = fmt.Sprintf("r%d", pythonRunner.nextRequestID.Add(1))
	}

	responseChannel := make(chan rpcResponse, 1)
	pythonRunner.pendingMutex.Lock()
	pythonRunner.pending[request.ID] = responseChannel
	pythonRunner.pendingMutex.Unlock()

	requestPayload, err := json.Marshal(request)
	if err != nil {
		pythonRunner.cancelPending(request.ID)
		return rpcResponse{}, fmt.Errorf("marshal request: %w", err)
	}
	requestPayload = append(requestPayload, '\n')

	pythonRunner.mutex.Lock()
	_, writeErr := pythonRunner.stdin.Write(requestPayload)
	pythonRunner.mutex.Unlock()
	if writeErr != nil {
		pythonRunner.cancelPending(request.ID)
		return rpcResponse{}, fmt.Errorf("stdin write: %w", writeErr)
	}

	select {
	case runnerResponse, ok := <-responseChannel:
		if !ok {
			return rpcResponse{}, ErrRunnerExited
		}
		return runnerResponse, nil
	case <-time.After(timeout):
		pythonRunner.cancelPending(request.ID)
		return rpcResponse{}, ErrRunnerTimeout
	}
}

func (pythonRunner *runner) cancelPending(requestID string) {
	pythonRunner.pendingMutex.Lock()
	if pendingChannel, exists := pythonRunner.pending[requestID]; exists {
		delete(pythonRunner.pending, requestID)
		close(pendingChannel)
	}
	pythonRunner.pendingMutex.Unlock()
}

func (pythonRunner *runner) close(shutdownGrace time.Duration) error {
	if pythonRunner.exited.Load() {
		return nil
	}

	_, _ = pythonRunner.send(rpcRequest{Op: "shutdown"}, shutdownGrace)
	_ = pythonRunner.stdin.Close()

	readerFinished := make(chan struct{})
	go func() {
		<-pythonRunner.readerDone
		close(readerFinished)
	}()

	select {
	case <-readerFinished:
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
		return fmt.Errorf("kill runner: %w", err)
	}
	return nil
}
