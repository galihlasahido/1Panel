package encrypt

import (
	"os"
	"path/filepath"
	"testing"
)

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

	if _, ok := readKeyFile(); ok {
		t.Fatalf("expected no key before write")
	}
	if err := writeKeyFile("secret-key-123"); err != nil {
		t.Fatalf("writeKeyFile: %v", err)
	}
	got, ok := readKeyFile()
	if !ok || got != "secret-key-123" {
		t.Fatalf("readKeyFile after write: ok=%v got=%q", ok, got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("key file mode = %o, want 0600", got)
	}
}

func TestKeyFile_EmptyKeyIsNoOp(t *testing.T) {
	withTempKeyPath(t)
	if err := writeKeyFile(""); err != nil {
		t.Fatalf("writeKeyFile empty: %v", err)
	}
	if _, ok := readKeyFile(); ok {
		t.Fatalf("expected empty write to be a no-op")
	}
}
