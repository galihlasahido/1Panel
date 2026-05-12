package encrypt

import (
	"os"
	"path/filepath"
)

// encryptKeyFilePath is the canonical on-disk location for the AES master key.
// Both 1panel-core and 1panel-agent read/write this file. Lives outside the
// SQLite database so that a leaked DB snapshot does not give an attacker the
// keying material needed to decrypt stored secrets.
var encryptKeyFilePath = "/etc/1panel/.encrypt_key"

func readKeyFile() (string, bool) {
	data, err := os.ReadFile(encryptKeyFilePath)
	if err != nil || len(data) == 0 {
		return "", false
	}
	return string(data), true
}

func writeKeyFile(key string) error {
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
