package enginehost

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"nstudio/app/common/daemon"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/tts"
	"nstudio/app/tts/modelManager"
)

var (
	watchInterval      = 2 * time.Second
	answerPollInterval = 250 * time.Millisecond
)

// ErrStartCancelled is returned by StartDaemon when StopDaemon stopped the server while it was
// still starting.
var ErrStartCancelled = errors.New("the server was stopped while it was starting")

var (
	mutex          sync.Mutex
	handedOff      bool
	starting       bool
	stopRequested  bool
	initializeOnce sync.Once
)

var (
	daemonRunning       = isDaemonRunning
	daemonAnswering     = isDaemonAnswering
	stopPlayback        = tts.Stop
	releaseLocalEngines = modelManager.ReleaseAll
	restartLocalEngines = modelManager.Reacquire
)

func Initialize() {
	initializeOnce.Do(func() {
		mutex.Lock()
		syncWithDaemon()
		mutex.Unlock()

		go watchDaemon()
	})
}

// StartDaemon returns once the server answers, however long loading its models takes, or with an
// error if it exits first. The wait doesn't hold the lock, so StopDaemon can cancel it.
func StartDaemon(launchDaemon func() error) error {
	mutex.Lock()
	if !handedOff {
		handEnginesToDaemon()
	}

	if err := launchDaemon(); err != nil {
		syncWithDaemon()
		mutex.Unlock()
		return err
	}
	starting, stopRequested = true, false
	mutex.Unlock()

	err := waitForDaemonToAnswer()

	mutex.Lock()
	defer mutex.Unlock()
	starting = false
	if err != nil {
		syncWithDaemon()
		if stopRequested {
			return ErrStartCancelled
		}
		return err
	}

	return nil
}

func StopDaemon(stopDaemon func() error) error {
	mutex.Lock()
	defer mutex.Unlock()

	if starting {
		stopRequested = true
	}
	err := stopDaemon()
	syncWithDaemon()
	return err
}

func watchDaemon() {
	ticker := time.NewTicker(watchInterval)
	for range ticker.C {
		mutex.Lock()
		syncWithDaemon()
		mutex.Unlock()
	}
}

// Taking the engines needs an answering daemon (a PID file can be stale, its PID
// reused); giving them back needs only a dead one, so a slow-loading daemon
// doesn't bounce them back and forth.
func syncWithDaemon() {
	running := daemonRunning()

	switch {
	case handedOff && !running:
		takeEnginesBack()
	case !handedOff && running && daemonAnswering():
		handEnginesToDaemon()
	}
}

func handEnginesToDaemon() {
	stopPlayback()
	releaseLocalEngines()
	handedOff = true
	response.Info(util.MessageData{Summary: "Engines handed to the background server"})
}

func takeEnginesBack() {
	handedOff = false
	_ = restartLocalEngines()
	response.Info(util.MessageData{Summary: "Engines taken back from the background server"})
}

func waitForDaemonToAnswer() error {
	for !daemonAnswering() {
		if !daemonRunning() {
			return response.Err("the server exited while starting; see the server log")
		}
		time.Sleep(answerPollInterval)
	}

	return nil
}

func isDaemonRunning() bool {
	running, _, err := daemon.GetDaemonStatus()
	return err == nil && running
}

var probeClient = &http.Client{Timeout: time.Second}

func isDaemonAnswering() bool {
	endpoint, err := daemon.ServerEndpoint()
	if err != nil {
		return false
	}

	infoResponse, err := probeClient.Get(endpoint + "/info")
	if err != nil {
		return false
	}
	defer infoResponse.Body.Close()

	var serverInfo struct {
		Name string `json:"name"`
	}
	if infoResponse.StatusCode != http.StatusOK || json.NewDecoder(infoResponse.Body).Decode(&serverInfo) != nil {
		return false
	}

	return serverInfo.Name == "Narration Studio API"
}
