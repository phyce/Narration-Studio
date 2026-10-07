package daemon

import (
	"os"
	"strconv"
	"testing"
	"time"
)

func useTemporaryStatusDirectory(t *testing.T) {
	t.Helper()
	temporaryDirectory := t.TempDir()
	t.Setenv("TMP", temporaryDirectory)
	t.Setenv("TEMP", temporaryDirectory)
	t.Setenv("TMPDIR", temporaryDirectory)

	lastWrittenStatus = nil
	t.Cleanup(func() { lastWrittenStatus = nil })
}

func testStatus() DaemonStatusInfo {
	return DaemonStatusInfo{
		PID:       4321,
		Version:   "1.2.3",
		StartTime: time.Unix(1700000000, 0).UTC(),
		Mode:      "http",
		Host:      "127.0.0.1",
		Port:      8080,
	}
}

// The daemon polls its own status on a timer for its whole lifetime, so a write
// that changes nothing must not reach the disk.
func TestWriteDaemonStatusSkipsUnchangedWrites(t *testing.T) {
	useTemporaryStatusDirectory(t)
	status := testStatus()

	if err := WriteDaemonStatus(status); err != nil {
		t.Fatalf("first write: %v", err)
	}

	// Replace the file with a sentinel: a redundant write would overwrite it.
	const sentinel = "untouched"
	statusPath := GetStatusFilePath()
	if err := os.WriteFile(statusPath, []byte(sentinel), 0644); err != nil {
		t.Fatalf("plant sentinel: %v", err)
	}

	if err := WriteDaemonStatus(status); err != nil {
		t.Fatalf("repeat write: %v", err)
	}

	current, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("read status file: %v", err)
	}
	if string(current) != sentinel {
		t.Error("writing an identical status rewrote the file")
	}
}

func TestWriteDaemonStatusPersistsChanges(t *testing.T) {
	useTemporaryStatusDirectory(t)
	status := testStatus()

	if err := WriteDaemonStatus(status); err != nil {
		t.Fatalf("first write: %v", err)
	}

	status.ProcessedMessages = 7
	if err := WriteDaemonStatus(status); err != nil {
		t.Fatalf("second write: %v", err)
	}

	stored, err := ReadDaemonStatus()
	if err != nil {
		t.Fatalf("read back status: %v", err)
	}
	if stored.ProcessedMessages != 7 {
		t.Errorf("ProcessedMessages = %d, want 7", stored.ProcessedMessages)
	}
	if stored.Mode != "http" || stored.Host != "127.0.0.1" || stored.Port != 8080 {
		t.Errorf("address fields not persisted: mode=%q host=%q port=%d", stored.Mode, stored.Host, stored.Port)
	}
}

// A temp directory cleaner wiping the file must not leave the daemon looking
// dead just because the in-process cache still matches.
func TestWriteDaemonStatusRewritesMissingFile(t *testing.T) {
	useTemporaryStatusDirectory(t)
	status := testStatus()

	if err := WriteDaemonStatus(status); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := os.Remove(GetStatusFilePath()); err != nil {
		t.Fatalf("remove status file: %v", err)
	}

	if err := WriteDaemonStatus(status); err != nil {
		t.Fatalf("rewrite after removal: %v", err)
	}

	if _, err := ReadDaemonStatus(); err != nil {
		t.Errorf("status file was not recreated: %v", err)
	}
}

func writeRunningDaemonFiles(t *testing.T, status DaemonStatusInfo) {
	t.Helper()
	if err := os.WriteFile(GetPidFilePath(), []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		t.Fatalf("write PID file: %v", err)
	}
	if err := WriteDaemonStatus(status); err != nil {
		t.Fatalf("write status file: %v", err)
	}
}

func TestServerEndpointNormalizesWildcardHosts(t *testing.T) {
	cases := map[string]string{
		"":          "http://127.0.0.1:8080",
		"0.0.0.0":   "http://127.0.0.1:8080",
		"::":        "http://127.0.0.1:8080",
		"[::]":      "http://127.0.0.1:8080",
		"localhost": "http://localhost:8080",
		"::1":       "http://[::1]:8080",
		"10.0.0.5":  "http://10.0.0.5:8080",
	}

	for listenHost, wantEndpoint := range cases {
		t.Run(listenHost, func(t *testing.T) {
			useTemporaryStatusDirectory(t)
			status := testStatus()
			status.PID = os.Getpid()
			status.Host = listenHost
			writeRunningDaemonFiles(t, status)

			endpoint, err := ServerEndpoint()
			if err != nil {
				t.Fatalf("ServerEndpoint: %v", err)
			}
			if endpoint != wantEndpoint {
				t.Errorf("endpoint = %q, want %q", endpoint, wantEndpoint)
			}
		})
	}
}

func TestServerEndpointRejectsStaleOrUnusableStatus(t *testing.T) {
	t.Run("NoDaemon", func(t *testing.T) {
		useTemporaryStatusDirectory(t)
		if _, err := ServerEndpoint(); err == nil {
			t.Error("found an endpoint with no daemon running")
		}
	})

	t.Run("StatusFromAnotherProcess", func(t *testing.T) {
		useTemporaryStatusDirectory(t)
		status := testStatus()
		status.PID = os.Getpid() + 1
		writeRunningDaemonFiles(t, status)

		if _, err := ServerEndpoint(); err == nil {
			t.Error("trusted a status file written by a different PID")
		}
	})

	t.Run("NotHTTPMode", func(t *testing.T) {
		useTemporaryStatusDirectory(t)
		status := testStatus()
		status.PID = os.Getpid()
		status.Mode = "websocket"
		writeRunningDaemonFiles(t, status)

		if _, err := ServerEndpoint(); err == nil {
			t.Error("returned an HTTP endpoint for a websocket-mode daemon")
		}
	})
}
