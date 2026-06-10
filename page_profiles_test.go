//go:build !cli && !clib

package main

import (
	"runtime"
	"strings"
	"testing"

	"nstudio/app/common/util"
	"nstudio/app/config"
	"nstudio/app/tts/profile"
)

func TestProfilesPage(t *testing.T) {
	app := NewApp()
	const profileID = "uitestprofile"

	t.Run("CreateProfile", func(t *testing.T) {
		result := app.CreateProfile(profileID, "UI Test Profile", "created by tests")
		if result == "" {
			t.Fatal("CreateProfile returned empty result")
		}
		mustParseJSON(t, "CreateProfile", result)
	})

	t.Run("GetProfiles", func(t *testing.T) {
		result := app.GetProfiles()
		mustParseJSON(t, "GetProfiles", result)
		if !strings.Contains(result, profileID) {
			t.Errorf("GetProfiles does not include the created profile %q: %s", profileID, result)
		}
	})

	t.Run("SaveAndGetProfileVoices", func(t *testing.T) {
		voices := `{"Hero":{"name":"Hero","engine":"openai","model":"tts-1","voice":"alloy"}}`
		app.SaveProfileVoices(profileID, voices)

		result := app.GetProfileVoices(profileID)
		mustParseJSON(t, "GetProfileVoices", result)
		if !strings.Contains(result, "Hero") {
			t.Errorf("GetProfileVoices did not reflect saved voice: %s", result)
		}
	})

	t.Run("SaveAndGetProfileSettings", func(t *testing.T) {
		current := app.GetProfileSettings(profileID)
		mustParseJSON(t, "GetProfileSettings", current)
		app.SaveProfileSettings(profileID, current)
		mustParseJSON(t, "GetProfileSettings(after save)", app.GetProfileSettings(profileID))
	})

	t.Run("GetProfileSettingsSchema", func(t *testing.T) {
		schema := app.GetProfileSettingsSchema()
		if schema == "" || schema == "{}" {
			t.Fatalf("GetProfileSettingsSchema returned empty schema: %q", schema)
		}
		mustParseJSON(t, "GetProfileSettingsSchema", schema)
	})

	t.Run("GetEnginesForProfile", func(t *testing.T) {
		result := app.GetEnginesForProfile(profileID)
		value := mustParseJSON(t, "GetEnginesForProfile", result)
		if _, ok := value.([]interface{}); !ok {
			t.Errorf("GetEnginesForProfile did not return a JSON array: %.120q", result)
		}
	})

	t.Run("EventTrigger", func(t *testing.T) {
		app.EventTrigger("ui.test.event", map[string]interface{}{"ok": true})
	})

	t.Run("DeleteProfile", func(t *testing.T) {
		app.DeleteProfile(profileID)
		if strings.Contains(app.GetProfiles(), profileID) {
			t.Errorf("DeleteProfile did not remove profile %q", profileID)
		}
	})
}

func TestDisabledVoiceReallocation(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("uses the SAPI5 engine (Windows only)")
	}

	base := config.Get()
	originalToggles := base.ModelToggles
	base.ModelToggles = map[string]bool{"mssapi5:mssapi5": true}
	if err := config.Set(base); err != nil {
		t.Fatalf("set toggles: %v", err)
	}
	t.Cleanup(func() {
		restore := config.Get()
		restore.ModelToggles = originalToggles
		_ = config.Set(restore)
	})

	const profileID = "uitestrealloc"
	manager := profile.GetManager()
	created, err := manager.CreateProfile(profileID, "Realloc Test", "")
	if err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	t.Cleanup(func() { _ = manager.DeleteProfile(profileID) })

	created.SetVoice("Hero", &util.CharacterVoice{Name: "Hero", Engine: "openai", Model: "tts-1", Voice: "alloy"})
	if err := manager.SaveProfile(created); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	voice, err := manager.GetOrAllocateVoice(profileID, "Hero")
	if err != nil {
		t.Fatalf("GetOrAllocateVoice: %v", err)
	}
	if voice.Engine == "openai" {
		t.Fatal("disabled openai voice was returned instead of a replacement")
	}
	if voice.Engine != "mssapi5" {
		t.Fatalf("replacement voice engine = %q, want mssapi5 (only enabled engine)", voice.Engine)
	}

	stored, err := manager.GetVoiceConfig(profileID, "Hero")
	if err != nil {
		t.Fatalf("GetVoiceConfig: %v", err)
	}
	if stored.Engine != voice.Engine || stored.Voice != voice.Voice {
		t.Fatalf("stored allocation was not overridden: stored %s:%s, returned %s:%s",
			stored.Engine, stored.Voice, voice.Engine, voice.Voice)
	}
}
