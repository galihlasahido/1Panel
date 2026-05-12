package encrypt

import (
	"os"
	"path/filepath"
	"testing"
)

// withTempKeyPath redirects encryptKeyFilePath to a per-test tmp dir and
// restores the original on cleanup.
func withTempKeyPath(t *testing.T) string {
	t.Helper()
	orig := encryptKeyFilePath
	dir := t.TempDir()
	encryptKeyFilePath = filepath.Join(dir, ".encrypt_key")
	t.Cleanup(func() { encryptKeyFilePath = orig })
	return encryptKeyFilePath
}

func TestKeyFile_RoundTrip(t *testing.T) {
	path := withTempKeyPath(t)

	if _, ok := ReadKeyFile(); ok {
		t.Fatalf("expected no key before write")
	}
	if err := WriteKeyFile("secret-key-123"); err != nil {
		t.Fatalf("WriteKeyFile: %v", err)
	}
	got, ok := ReadKeyFile()
	if !ok || got != "secret-key-123" {
		t.Fatalf("ReadKeyFile after write: ok=%v got=%q", ok, got)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("key file mode = %o, want 0600", got)
	}
}

func TestKeyFile_AtomicWrite(t *testing.T) {
	withTempKeyPath(t)
	if err := WriteKeyFile("first"); err != nil {
		t.Fatalf("WriteKeyFile: %v", err)
	}
	// Overwriting should not leave the temp file behind.
	if err := WriteKeyFile("second"); err != nil {
		t.Fatalf("WriteKeyFile: %v", err)
	}
	if _, err := os.Stat(encryptKeyFilePath + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("expected .tmp removed after rename, stat err=%v", err)
	}
	got, _ := ReadKeyFile()
	if got != "second" {
		t.Fatalf("got %q want second", got)
	}
}

func TestKeyFile_EmptyKeyIsNoOp(t *testing.T) {
	withTempKeyPath(t)
	if err := WriteKeyFile(""); err != nil {
		t.Fatalf("WriteKeyFile empty: %v", err)
	}
	if _, ok := ReadKeyFile(); ok {
		t.Fatalf("expected empty write to be a no-op")
	}
}
