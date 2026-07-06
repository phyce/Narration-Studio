package pack

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeThroughForTest(t *testing.T, data []byte, expectedSHA256 string) error {
	t.Helper()
	destinationPath := filepath.Join(t.TempDir(), "bundle.nstudio")
	return writeThroughTemporaryFile(strings.NewReader(string(data)), destinationPath, expectedSHA256)
}

func TestWriteThroughTemporaryFile_DigestMatch(t *testing.T) {
	data := []byte("hello bundle")
	sum := sha256.Sum256(data)
	if err := writeThroughForTest(t, data, hex.EncodeToString(sum[:])); err != nil {
		t.Fatalf("expected matching digest to succeed, got: %v", err)
	}
}

func TestWriteThroughTemporaryFile_DigestMismatchAborts(t *testing.T) {
	destinationPath := filepath.Join(t.TempDir(), "bundle.nstudio")
	badDigest := strings.Repeat("0", 64)
	err := writeThroughTemporaryFile(strings.NewReader("hello bundle"), destinationPath, badDigest)
	if err == nil {
		t.Fatal("expected digest mismatch error, got nil")
	}
	if _, statErr := os.Stat(destinationPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected no installed file on mismatch, stat err: %v", statErr)
	}
	if _, statErr := os.Stat(destinationPath + ".part"); !os.IsNotExist(statErr) {
		t.Fatal("expected temp .part file to be removed on mismatch")
	}
}

func TestWriteThroughTemporaryFile_EmptyDigestProceeds(t *testing.T) {
	if err := writeThroughForTest(t, []byte("unverified"), ""); err != nil {
		t.Fatalf("expected empty digest to proceed unverified, got: %v", err)
	}
}

func TestDownloadBundle_RejectsInsecureURL(t *testing.T) {
	destinationPath := filepath.Join(t.TempDir(), "bundle.nstudio")
	progress := &progressReader{engineID: "test"}
	err := downloadBundle("http://example.com/pack.nstudio", destinationPath, "", progress)
	if err == nil {
		t.Fatal("expected http:// URL to be rejected")
	}
	if !strings.Contains(err.Error(), "https required") {
		t.Fatalf("expected https-required error, got: %v", err)
	}
}

func TestParseSHA256Digest(t *testing.T) {
	cases := map[string]string{
		"sha256:ABCDEF": "abcdef",
		"SHA256:abc123": "abc123",
		"":              "",
		"md5:abc":       "",
		"abcdef":        "",
	}
	for input, want := range cases {
		if got := parseSHA256Digest(input); got != want {
			t.Errorf("parseSHA256Digest(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestValidateBundleRelativePath(t *testing.T) {
	valid := []string{"bin/python", "python3", "runtime/bin/python3.11"}
	for _, p := range valid {
		if err := validateBundleRelativePath(p); err != nil {
			t.Errorf("expected %q to be valid, got: %v", p, err)
		}
	}
	invalid := []string{"/usr/bin/python", "../python", "bin/../../python", "a/../../../etc/passwd"}
	for _, p := range invalid {
		if err := validateBundleRelativePath(p); err == nil {
			t.Errorf("expected %q to be rejected", p)
		}
	}
}
