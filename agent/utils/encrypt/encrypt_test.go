package encrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"strings"
	"testing"
)

const testKey = "abcdefghijklmnop"

func TestGCMRoundTrip(t *testing.T) {
	plain := "hello agent secret"
	ct, err := StringEncryptWithKey(plain, testKey)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if !strings.HasPrefix(ct, v2Prefix) {
		t.Fatalf("expected v2 prefix, got %q", ct)
	}
	pt, err := StringDecryptWithKey(ct, testKey)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if pt != plain {
		t.Fatalf("roundtrip mismatch: %q != %q", pt, plain)
	}
}

func TestGCMRejectsTampering(t *testing.T) {
	ct, err := StringEncryptWithKey("authenticated", testKey)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	raw, _ := base64.StdEncoding.DecodeString(ct[len(v2Prefix):])
	raw[len(raw)-1] ^= 0x01
	tampered := v2Prefix + base64.StdEncoding.EncodeToString(raw)
	if _, err := StringDecryptWithKey(tampered, testKey); err == nil {
		t.Fatalf("expected GCM to reject tampered ciphertext")
	}
}

func TestLegacyCBCDecryptStillWorks(t *testing.T) {
	plain := "legacy agent data"
	legacyCT, err := legacyEncryptForTest(testKey, plain)
	if err != nil {
		t.Fatalf("legacy encrypt: %v", err)
	}
	pt, err := StringDecryptWithKey(legacyCT, testKey)
	if err != nil {
		t.Fatalf("legacy decrypt: %v", err)
	}
	if pt != plain {
		t.Fatalf("legacy roundtrip mismatch: %q != %q", pt, plain)
	}
}

func TestLegacyShortKeyExpansion(t *testing.T) {
	plain := "x"
	legacyCT, err := legacyEncryptForTest("short"+strings.Repeat("u", 11), plain)
	if err != nil {
		t.Fatalf("legacy encrypt: %v", err)
	}
	pt, err := StringDecryptWithKey(legacyCT, "short")
	if err != nil {
		t.Fatalf("legacy short-key decrypt: %v", err)
	}
	if pt != plain {
		t.Fatalf("expected %q, got %q", plain, pt)
	}
}

func legacyEncryptForTest(key, plain string) (string, error) {
	k := []byte(key)
	if len(k) < 16 {
		return "", io.ErrUnexpectedEOF
	}
	k = k[:16]
	block, err := aes.NewCipher(k)
	if err != nil {
		return "", err
	}
	pt := []byte(plain)
	padLen := aes.BlockSize - len(pt)%aes.BlockSize
	for i := 0; i < padLen; i++ {
		pt = append(pt, byte(padLen))
	}
	buf := make([]byte, aes.BlockSize+len(pt))
	iv := buf[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(buf[aes.BlockSize:], pt)
	return base64.StdEncoding.EncodeToString(buf), nil
}
