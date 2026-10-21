package modelManager

import (
	"context"
	"errors"
	"testing"

	tts "nstudio/app/tts/engine"
)

type stubApiEngine struct {
	stubEngine
	availabilityErr error
	checked         bool
}

func (engine *stubApiEngine) CheckAvailability(context.Context) error {
	engine.checked = true
	return engine.availabilityErr
}

func stubApiEntry(engineType tts.EngineType, checker *stubApiEngine) *EngineEntry {
	return &EngineEntry{
		Engine: tts.Engine{Type: engineType, Engine: checker},
		Models: make(map[string]*ModelPool),
	}
}

func TestCheckApiEnginesChecksEveryApiEngine(t *testing.T) {
	localEngine := &stubApiEngine{}
	useTestManager(t, map[string]*EngineEntry{
		"working": stubApiEntry(tts.Api, &stubApiEngine{}),
		"keyless": stubApiEntry(tts.Api, &stubApiEngine{availabilityErr: tts.ErrApiKeyNotSet}),
		"refused": stubApiEntry(tts.Api, &stubApiEngine{availabilityErr: errors.New("Refused refused the API key")}),
		"local":   stubApiEntry(tts.Local, localEngine),
	})

	availability := CheckApiEngines(context.Background())

	expected := map[string]ApiAvailability{
		"working": {Available: true},
		"keyless": {MissingApiKey: true, Reason: tts.ErrApiKeyNotSet.Error()},
		"refused": {Reason: "Refused refused the API key"},
	}
	if len(availability) != len(expected) {
		t.Errorf("checked %v, want only the API engines", availability)
	}
	for engineID, expectedAvailability := range expected {
		if availability[engineID] != expectedAvailability {
			t.Errorf("%s: got %+v, want %+v", engineID, availability[engineID], expectedAvailability)
		}
	}
	if localEngine.checked {
		t.Error("a local engine was checked")
	}
}
