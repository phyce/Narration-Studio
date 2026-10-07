package pack

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrInstallPaused     = errors.New("install paused")
	ErrInstallCancelled  = errors.New("install cancelled")
	ErrAlreadyInstalling = errors.New("this engine is already being installed")
	ErrServerRunning     = errors.New("the background server is using the engine packs; stop the server to install or remove them")
)

var runningInstalls = struct {
	sync.Mutex
	stopByEngineID map[string]context.CancelCauseFunc
}{stopByEngineID: make(map[string]context.CancelCauseFunc)}

func beginInstall(engineID string) (context.Context, func(), error) {
	runningInstalls.Lock()
	defer runningInstalls.Unlock()

	if _, running := runningInstalls.stopByEngineID[engineID]; running {
		return nil, nil, ErrAlreadyInstalling
	}
	installContext, stop := context.WithCancelCause(context.Background())
	runningInstalls.stopByEngineID[engineID] = stop

	finish := func() {
		runningInstalls.Lock()
		delete(runningInstalls.stopByEngineID, engineID)
		runningInstalls.Unlock()
		stop(nil)
	}
	return installContext, finish, nil
}

func isInstalling(engineID string) bool {
	runningInstalls.Lock()
	defer runningInstalls.Unlock()
	_, running := runningInstalls.stopByEngineID[engineID]
	return running
}

func PauseInstall(engineID string) bool {
	return stopInstall(engineID, ErrInstallPaused)
}

func CancelInstall(engineID string) bool {
	return stopInstall(engineID, ErrInstallCancelled)
}

func stopInstall(engineID string, reason error) bool {
	runningInstalls.Lock()
	defer runningInstalls.Unlock()
	stop, running := runningInstalls.stopByEngineID[engineID]
	if running {
		stop(reason)
	}
	return running
}
