package enginehost

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"nstudio/app/common/daemon"
	"nstudio/app/tts/engine/enginetest"
)

func TestMain(m *testing.M) {
	os.Exit(enginetest.RunWithTemporaryConfig(m))
}

type fakeDaemon struct {
	lock              sync.Mutex
	running           bool
	answering         bool
	runningChecks     int
	diesAfterCheck    int
	answerChecks      int
	answersAfterCheck int
	playbackStops     int
	releases          int
	restarts          int
}

func (fake *fakeDaemon) setRunning(running bool) {
	fake.lock.Lock()
	defer fake.lock.Unlock()
	fake.running = running
}

func useFakeDaemon(t *testing.T, startHandedOff bool) *fakeDaemon {
	t.Helper()
	fake := &fakeDaemon{}

	previousRunning, previousAnswering := daemonRunning, daemonAnswering
	previousStop, previousRelease, previousRestart := stopPlayback, releaseLocalEngines, restartLocalEngines
	previousPoll := answerPollInterval
	t.Cleanup(func() {
		daemonRunning, daemonAnswering = previousRunning, previousAnswering
		stopPlayback, releaseLocalEngines, restartLocalEngines = previousStop, previousRelease, previousRestart
		answerPollInterval = previousPoll
		handedOff = false
	})

	daemonRunning = func() bool {
		fake.lock.Lock()
		defer fake.lock.Unlock()
		fake.runningChecks++
		if fake.diesAfterCheck > 0 && fake.runningChecks >= fake.diesAfterCheck {
			fake.running = false
		}
		return fake.running
	}
	daemonAnswering = func() bool {
		fake.lock.Lock()
		defer fake.lock.Unlock()
		fake.answerChecks++
		if fake.answersAfterCheck > 0 && fake.answerChecks >= fake.answersAfterCheck {
			fake.answering = true
		}
		return fake.running && fake.answering
	}
	stopPlayback = func() { fake.playbackStops++ }
	releaseLocalEngines = func() { fake.releases++ }
	restartLocalEngines = func() error {
		fake.lock.Lock()
		defer fake.lock.Unlock()
		fake.restarts++
		return nil
	}
	answerPollInterval = 5 * time.Millisecond
	handedOff = startHandedOff

	return fake
}

func TestStartDaemonReleasesEnginesBeforeLaunching(t *testing.T) {
	fake := useFakeDaemon(t, false)

	err := StartDaemon(func() error {
		if fake.releases != 1 || fake.playbackStops != 1 {
			t.Errorf("launched with releases=%d playbackStops=%d, want both 1", fake.releases, fake.playbackStops)
		}
		fake.running, fake.answering = true, true
		return nil
	})

	if err != nil {
		t.Fatalf("StartDaemon: %v", err)
	}
	if !handedOff || fake.restarts != 0 {
		t.Errorf("handedOff=%v restarts=%d, want true and 0", handedOff, fake.restarts)
	}
}

func TestStartDaemonTakesEnginesBackWhenLaunchFails(t *testing.T) {
	fake := useFakeDaemon(t, false)

	err := StartDaemon(func() error { return errors.New("cli not found") })

	if err == nil {
		t.Fatal("StartDaemon reported success for a failed launch")
	}
	if handedOff || fake.restarts != 1 {
		t.Errorf("handedOff=%v restarts=%d, want false and 1", handedOff, fake.restarts)
	}
}

func TestStartDaemonKeepsEnginesWithAlreadyRunningDaemon(t *testing.T) {
	fake := useFakeDaemon(t, false)
	fake.running, fake.answering = true, true

	err := StartDaemon(func() error { return errors.New("daemon is already running") })

	if err == nil {
		t.Fatal("StartDaemon reported success for a failed launch")
	}
	if !handedOff || fake.restarts != 0 {
		t.Errorf("handedOff=%v restarts=%d, want true and 0: the running daemon still hosts the engines", handedOff, fake.restarts)
	}
}

func TestStartDaemonTakesEnginesBackWhenDaemonDiesWhileStarting(t *testing.T) {
	fake := useFakeDaemon(t, false)

	err := StartDaemon(func() error {
		fake.running = true
		fake.diesAfterCheck = fake.runningChecks + 3
		return nil
	})

	if err == nil {
		t.Fatal("StartDaemon reported success for a daemon that died")
	}
	if handedOff || fake.restarts != 1 {
		t.Errorf("handedOff=%v restarts=%d, want false and 1", handedOff, fake.restarts)
	}
}

func TestStartDaemonWaitsForADaemonThatIsStillLoading(t *testing.T) {
	fake := useFakeDaemon(t, false)

	err := StartDaemon(func() error {
		fake.running = true
		fake.answersAfterCheck = 100
		return nil
	})

	if err != nil {
		t.Fatalf("StartDaemon gave up on a daemon that was still loading: %v", err)
	}
	if fake.answerChecks < 100 {
		t.Errorf("returned after %d answer checks, want it to wait for the daemon to answer", fake.answerChecks)
	}
	if !handedOff || fake.restarts != 0 {
		t.Errorf("handedOff=%v restarts=%d, want true and 0", handedOff, fake.restarts)
	}
}

func TestStopDaemonCancelsAStartThatIsStillLoading(t *testing.T) {
	fake := useFakeDaemon(t, false)
	launched := make(chan struct{})
	startResult := make(chan error, 1)

	go func() {
		startResult <- StartDaemon(func() error {
			fake.setRunning(true)
			close(launched)
			return nil
		})
	}()
	<-launched

	stopResult := make(chan error, 1)
	go func() {
		stopResult <- StopDaemon(func() error {
			fake.setRunning(false)
			return nil
		})
	}()

	select {
	case err := <-stopResult:
		if err != nil {
			t.Fatalf("StopDaemon: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("StopDaemon waited for the start to finish")
	}

	select {
	case err := <-startResult:
		if !errors.Is(err, ErrStartCancelled) {
			t.Errorf("StartDaemon = %v, want ErrStartCancelled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("StartDaemon kept waiting after the daemon was stopped")
	}

	mutex.Lock()
	defer mutex.Unlock()
	fake.lock.Lock()
	defer fake.lock.Unlock()
	if handedOff || fake.restarts != 1 {
		t.Errorf("handedOff=%v restarts=%d, want false and 1", handedOff, fake.restarts)
	}
}

func TestStopDaemonTakesEnginesBack(t *testing.T) {
	fake := useFakeDaemon(t, true)
	fake.running, fake.answering = true, true

	if err := StopDaemon(func() error {
		fake.running = false
		return nil
	}); err != nil {
		t.Fatalf("StopDaemon: %v", err)
	}

	if handedOff || fake.restarts != 1 {
		t.Errorf("handedOff=%v restarts=%d, want false and 1", handedOff, fake.restarts)
	}
}

func TestStopDaemonFailureKeepsEnginesHandedOff(t *testing.T) {
	fake := useFakeDaemon(t, true)
	fake.running, fake.answering = true, true

	if err := StopDaemon(func() error { return errors.New("access denied") }); err == nil {
		t.Fatal("StopDaemon swallowed the stop failure")
	}

	if !handedOff || fake.restarts != 0 {
		t.Errorf("handedOff=%v restarts=%d, want true and 0", handedOff, fake.restarts)
	}
}

func TestSyncWithDaemon(t *testing.T) {
	cases := []struct {
		name          string
		handedOff     bool
		running       bool
		answering     bool
		wantHandedOff bool
	}{
		{name: "DaemonAppearsAndAnswers", running: true, answering: true, wantHandedOff: true},
		{name: "StalePIDNeverAnswers", running: true, answering: false, wantHandedOff: false},
		{name: "NoDaemon", wantHandedOff: false},
		{name: "DaemonGone", handedOff: true, wantHandedOff: false},
		{name: "DaemonBusyButAlive", handedOff: true, running: true, answering: false, wantHandedOff: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fake := useFakeDaemon(t, testCase.handedOff)
			fake.running, fake.answering = testCase.running, testCase.answering

			syncWithDaemon()

			if handedOff != testCase.wantHandedOff {
				t.Errorf("handedOff = %v, want %v", handedOff, testCase.wantHandedOff)
			}
			if testCase.wantHandedOff && !testCase.handedOff && fake.playbackStops != 1 {
				t.Errorf("handing off stopped playback %d times, want 1", fake.playbackStops)
			}
		})
	}
}

func serveDaemonInfo(t *testing.T, serverName string) {
	t.Helper()
	temporaryPath := t.TempDir()
	t.Setenv("TMP", temporaryPath)
	t.Setenv("TEMP", temporaryPath)
	t.Setenv("TMPDIR", temporaryPath)

	infoServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/info" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"name":"` + serverName + `"}`))
	}))
	t.Cleanup(infoServer.Close)

	serverHost, serverPort, err := net.SplitHostPort(infoServer.Listener.Addr().String())
	if err != nil {
		t.Fatalf("split test server address: %v", err)
	}
	port, err := strconv.Atoi(serverPort)
	if err != nil {
		t.Fatalf("parse test server port: %v", err)
	}

	if err := os.WriteFile(daemon.GetPidFilePath(), []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		t.Fatalf("write PID file: %v", err)
	}
	if err := daemon.WriteDaemonStatus(daemon.DaemonStatusInfo{PID: os.Getpid(), Mode: "http", Host: serverHost, Port: port}); err != nil {
		t.Fatalf("write status file: %v", err)
	}
}

func TestIsDaemonAnswering(t *testing.T) {
	t.Run("NarrationStudioServer", func(t *testing.T) {
		serveDaemonInfo(t, "Narration Studio API")
		if !isDaemonAnswering() {
			t.Error("did not recognise a Narration Studio server answering at the status file's address")
		}
	})

	t.Run("SomeOtherServer", func(t *testing.T) {
		serveDaemonInfo(t, "Something Else")
		if isDaemonAnswering() {
			t.Error("mistook an unrelated server for the daemon")
		}
	})
}
