package encrypt

import (
	"os"
	"path/filepath"
)

// encryptKeyFilePath is the canonical on-disk location for the AES master key.
// Both 1panel-core and 1panel-agent read/write this file. Lives outside the
// SQLite database so that a leaked DB snapshot (e.g. a backup that captured
// /opt/1panel/db/core.db without /etc/1panel) does not give an attacker the
// keying material needed to decrypt stored secrets.
//
// Declared as a var so tests can redirect to a temporary directory.
var encryptKeyFilePath = "/etc/1panel/.encrypt_key"

// SetKeyFilePath overrides the on-disk path used by ReadKeyFile and
// WriteKeyFile. Apps with their own filesystem layout (e.g. Domus
// using /etc/domus/.encrypt_key) call this during init before any
// encrypt operation runs. Default is /etc/1panel/.encrypt_key.
func SetKeyFilePath(p string) { encryptKeyFilePath = p }

// ReadKeyFile returns the encryption key stored on disk, if any.
// The second return is false when the file does not exist or is empty.
func ReadKeyFile() (string, bool) {
	data, err := os.ReadFile(encryptKeyFilePath)
	if err != nil || len(data) == 0 {
		return "", false
	}
	return string(data), true
}

// WriteKeyFile persists key to the canonical on-disk location with mode
// 0600 (atomic via tmp+rename). Best-effort: failures (e.g. /etc/1panel
// not yet created on a fresh install) are not fatal because the DB-backed
// key remains the source of truth.
func WriteKeyFile(key string) error {
	if key == "" {
		return nil
	}
	dir := filepath.Dir(encryptKeyFilePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := encryptKeyFilePath + ".tmp"
	if err := os.WriteFile(tmp, []byte(key), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, encryptKeyFilePath)
}
