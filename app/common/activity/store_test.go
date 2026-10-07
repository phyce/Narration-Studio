package activity

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestCreateSessionFileUsesADatedFolderAndAvoidsCollisions(t *testing.T) {
	logsPath := t.TempDir()
	started := time.Date(2026, 10, 7, 22, 36, 5, 0, time.Local)

	first := createSessionFile(logsPath, started)
	second := createSessionFile(logsPath, started)

	if want := filepath.Join(logsPath, "2026-10-07", "22-36-05.jsonl"); first != want {
		t.Errorf("first session file = %s, want %s", first, want)
	}
	if want := filepath.Join(logsPath, "2026-10-07", "22-36-05-2.jsonl"); second != want {
		t.Errorf("second session file in the same second = %s, want %s", second, want)
	}
	for _, path := range []string{first, second} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("session file %s was not created: %v", path, err)
		}
	}
}

func TestRemoveExpiredLogsKeepsTheLastDays(t *testing.T) {
	logsPath := t.TempDir()
	now := time.Date(2026, 10, 7, 9, 0, 0, 0, time.Local)
	for _, name := range []string{"2026-10-07", "2026-10-06", "2026-10-05", "2026-09-27", "notes"} {
		if err := os.MkdirAll(filepath.Join(logsPath, name), 0755); err != nil {
			t.Fatal(err)
		}
	}

	removeExpiredLogs(logsPath, 2, now)

	if got, want := folderNames(t, logsPath), []string{"2026-10-06", "2026-10-07", "notes"}; !equalNames(got, want) {
		t.Errorf("folders left = %v, want %v", got, want)
	}
}

func TestMoveLegacyLogsIntoTheirDateFolder(t *testing.T) {
	configPath := t.TempDir()
	logsPath := filepath.Join(configPath, logsFolderName)
	legacyPath := filepath.Join(configPath, legacyLogFileName)
	if err := os.WriteFile(legacyPath, []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	lastWrite := time.Date(2026, 10, 3, 12, 0, 0, 0, time.Local)
	if err := os.Chtimes(legacyPath, lastWrite, lastWrite); err != nil {
		t.Fatal(err)
	}

	moveLegacyLogs(configPath, logsPath)

	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Errorf("legacy log is still in the config folder (stat error: %v)", err)
	}
	if _, err := os.Stat(filepath.Join(logsPath, "2026-10-03", "legacy-activity.jsonl")); err != nil {
		t.Errorf("legacy log was not moved into its date folder: %v", err)
	}
}

func folderNames(t *testing.T, path string) []string {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

func equalNames(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
