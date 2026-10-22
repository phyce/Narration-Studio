package modelManager

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"nstudio/app/common/audio"
	tts "nstudio/app/tts/engine"
	"nstudio/app/tts/engine/enginetest"
)

func TestMain(m *testing.M) {
	os.Exit(enginetest.RunWithTemporaryConfig(m))
}

type stubEngine struct {
	mutex   sync.Mutex
	stopped []string
}

func (engine *stubEngine) Initialize() error            { return nil }
func (engine *stubEngine) Start(modelName string) error { return nil }

func (engine *stubEngine) Stop(modelName string) error {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	engine.stopped = append(engine.stopped, modelName)
	return nil
}

func (engine *stubEngine) Synthesize(context.Context, tts.SpeechRequest) (*audio.Stream, error) {
	return nil, nil
}

func (engine *stubEngine) GetVoices(string) ([]tts.Voice, error) { return nil, nil }
func (engine *stubEngine) FetchModels() map[string]tts.Model     { return nil }

func (engine *stubEngine) stopCount() int {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	return len(engine.stopped)
}

func newTestPool(instances ...tts.Base) *ModelPool {
	return newModelPool(instances)
}

func newStubEntry(pools map[string]*ModelPool, modelIDs ...string) *EngineEntry {
	models := make(map[string]tts.Model, len(modelIDs))
	for _, modelID := range modelIDs {
		models[modelID] = tts.Model{ID: modelID, Name: modelID, Engine: "stub"}
	}
	if pools == nil {
		pools = make(map[string]*ModelPool)
	}

	return &EngineEntry{
		Engine: tts.Engine{
			ID:          "stub",
			Type:        tts.Local,
			Models:      models,
			Engine:      &stubEngine{},
			NewInstance: func() tts.Base { return &stubEngine{} },
		},
		Models: pools,
	}
}

func shortenDrainTimeout(t *testing.T, timeout time.Duration) {
	t.Helper()
	previous := releaseDrainTimeout
	releaseDrainTimeout = timeout
	t.Cleanup(func() { releaseDrainTimeout = previous })
}

func waitUntil(t *testing.T, condition func() bool, failure string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal(failure)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func useTestManager(t *testing.T, engines map[string]*EngineEntry) {
	t.Helper()
	previous := manager
	manager = &modelManager{Engines: engines, IsGUIMode: true}
	t.Cleanup(func() { manager = previous })
}

func TestReleaseAllStopsAndDetachesPools(t *testing.T) {
	firstInstance := &stubEngine{}
	secondInstance := &stubEngine{}
	entry := &EngineEntry{
		Engine: tts.Engine{ID: "stub", Type: tts.Local},
		Models: map[string]*ModelPool{
			"alpha": newTestPool(firstInstance),
			"beta":  newTestPool(secondInstance),
		},
	}
	useTestManager(t, map[string]*EngineEntry{"stub": entry})

	ReleaseAll()

	if len(entry.Models) != 0 {
		t.Errorf("entry still holds %d pools after ReleaseAll", len(entry.Models))
	}
	if firstInstance.stopCount() != 1 {
		t.Errorf("alpha stop count = %d, want 1", firstInstance.stopCount())
	}
	if secondInstance.stopCount() != 1 {
		t.Errorf("beta stop count = %d, want 1", secondInstance.stopCount())
	}
}

func TestReleaseAllWaitsForInFlightInstance(t *testing.T) {
	instance := &stubEngine{}
	pool := newTestPool(instance)
	entry := &EngineEntry{
		Engine: tts.Engine{ID: "stub", Type: tts.Local},
		Models: map[string]*ModelPool{"alpha": pool},
	}
	useTestManager(t, map[string]*EngineEntry{"stub": entry})

	checkedOut, ok := pool.GetNext()
	if !ok {
		t.Fatal("could not check an instance out")
	}

	released := make(chan struct{})
	go func() {
		ReleaseAll()
		close(released)
	}()

	select {
	case <-released:
		t.Fatal("ReleaseAll returned while an instance was still checked out")
	case <-time.After(150 * time.Millisecond):
	}
	if instance.stopCount() != 0 {
		t.Fatalf("instance was stopped while still in use (stop count %d)", instance.stopCount())
	}

	pool.Release(checkedOut)

	select {
	case <-released:
	case <-time.After(5 * time.Second):
		t.Fatal("ReleaseAll did not finish after the instance was handed back")
	}
	if instance.stopCount() != 1 {
		t.Errorf("stop count = %d, want 1", instance.stopCount())
	}
}

func TestDrainTimesOutOnStuckInstance(t *testing.T) {
	pool := newTestPool(&stubEngine{}, &stubEngine{})
	stuck, ok := pool.GetNext()
	if !ok {
		t.Fatal("could not check an instance out")
	}

	start := time.Now()
	recovered := pool.drain(200 * time.Millisecond)
	elapsed := time.Since(start)

	if len(recovered) != 1 {
		t.Errorf("recovered %d instances, want 1", len(recovered))
	}
	if elapsed < 200*time.Millisecond {
		t.Errorf("drain gave up after %v, want it to wait out the full timeout", elapsed)
	}

	pool.Release(stuck)
}

func TestReleaseAllWithNothingRegistered(t *testing.T) {
	useTestManager(t, map[string]*EngineEntry{})
	ReleaseAll()
}

func TestGetNextGivesUpWhenPoolIsDetached(t *testing.T) {
	pool := newTestPool(&stubEngine{})
	if _, ok := pool.GetNext(); !ok {
		t.Fatal("could not check the only instance out")
	}

	waiterResult := make(chan bool, 1)
	go func() {
		_, ok := pool.GetNext()
		waiterResult <- ok
	}()

	select {
	case <-waiterResult:
		t.Fatal("GetNext returned while the only instance was checked out")
	case <-time.After(100 * time.Millisecond):
	}

	pool.detach()

	select {
	case ok := <-waiterResult:
		if ok {
			t.Error("GetNext handed out an instance from a detached pool")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("GetNext stayed blocked after the pool was detached")
	}
}

func TestReleaseAllStopsInstancesReturnedAfterTimeout(t *testing.T) {
	shortenDrainTimeout(t, 100*time.Millisecond)

	instance := &stubEngine{}
	pool := newTestPool(instance)
	useTestManager(t, map[string]*EngineEntry{"stub": newStubEntry(map[string]*ModelPool{"alpha": pool}, "alpha")})

	checkedOut, ok := pool.GetNext()
	if !ok {
		t.Fatal("could not check an instance out")
	}

	ReleaseAll()
	if instance.stopCount() != 0 {
		t.Fatalf("instance was stopped while still in use (stop count %d)", instance.stopCount())
	}

	pool.Release(checkedOut)
	waitUntil(t, func() bool { return instance.stopCount() == 1 }, "instance handed back after the drain timed out was never stopped")
}

func TestReleasedManagerBuildsNoPoolsUntilReacquire(t *testing.T) {
	enginetest.EnableModel(t, "stub:alpha")
	enginetest.EnableModel(t, "second:gamma")

	entry := newStubEntry(nil, "alpha", "beta")
	useTestManager(t, map[string]*EngineEntry{"stub": entry})

	ReleaseAll()

	if _, _, ok := GetEngineInstance("stub", "alpha"); ok {
		t.Error("GetEngineInstance handed out an instance while released")
	}
	if _, _, ok := GetEngineInstance("stub", "beta"); ok {
		t.Error("GetEngineInstance created an on-demand pool while released")
	}
	if err := RefreshModels(); err != nil {
		t.Errorf("RefreshModels while released: %v", err)
	}
	if err := RegisterEngine(tts.Engine{
		ID:     "second",
		Type:   tts.Local,
		Engine: &stubEngine{},
		Models: map[string]tts.Model{"gamma": {ID: "gamma", Name: "gamma", Engine: "second"}},
		NewInstance: func() tts.Base {
			return &stubEngine{}
		},
	}); err != nil {
		t.Fatalf("RegisterEngine while released: %v", err)
	}

	if len(entry.Models) != 0 {
		t.Errorf("stub engine has %d pools while released, want 0", len(entry.Models))
	}
	if pools := manager.Engines["second"].Models; len(pools) != 0 {
		t.Errorf("engine registered while released has %d pools, want 0", len(pools))
	}

	if err := Reacquire(); err != nil {
		t.Fatalf("Reacquire: %v", err)
	}
	if IsReleased() {
		t.Error("still released after Reacquire")
	}
	if _, rebuilt := entry.Models["alpha"]; !rebuilt {
		t.Error("Reacquire did not rebuild the toggled model's pool")
	}

	_, release, ok := GetEngineInstance("stub", "alpha")
	if !ok {
		t.Fatal("no instance after Reacquire")
	}
	release()
}

func TestGetEngineInstanceRacesNothingDuringReleaseAll(t *testing.T) {
	enginetest.EnableModel(t, "stub:alpha")

	useTestManager(t, map[string]*EngineEntry{"stub": newStubEntry(nil, "alpha", "beta")})
	if err := Reacquire(); err != nil {
		t.Fatalf("Reacquire: %v", err)
	}

	stopRequesting := make(chan struct{})
	var requesters sync.WaitGroup
	for _, modelID := range []string{"alpha", "beta"} {
		requesters.Add(1)
		go func() {
			defer requesters.Done()
			for {
				select {
				case <-stopRequesting:
					return
				default:
				}
				if _, release, ok := GetEngineInstance("stub", modelID); ok {
					release()
				}
			}
		}()
	}

	for range 100 {
		ReleaseAll()
		if err := Reacquire(); err != nil {
			t.Fatalf("Reacquire: %v", err)
		}
	}

	close(stopRequesting)
	requesters.Wait()
}
