// Package encrypt provides AES-GCM (preferred, v2) and AES-CBC (legacy
// back-compat) symmetric encryption + RSA helpers used by the 1Panel
// family of products.
//
// The package is designed to be embedded by host applications that
// supply their own master-key resolution policy. Callers MUST register
// a KeyProvider via SetKeyProvider during init before StringEncrypt or
// StringDecrypt are called; without a provider, those functions return
// an error.
//
// Ciphertext format:
//   - v2 (preferred): "v2:" + base64(nonce(12) || GCM-sealed-plaintext-with-tag)
//   - v1 (legacy):     base64(IV(16) || AES-128-CBC(PKCS7-pad(plaintext)))
//
// StringDecrypt auto-detects format by the "v2:" prefix.
package encrypt

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"strings"
)

// v2Prefix marks ciphertext encrypted with AES-256-GCM (authenticated).
// Values without this prefix are legacy AES-128-CBC + PKCS7 from earlier
// 1Panel releases; they continue to decrypt for backward compatibility
// but new encryptions always write v2.
const v2Prefix = "v2:"

// KeyProvider returns the application's master encryption key. The same
// provider is invoked on every StringEncrypt/StringDecrypt call so apps
// can rotate the in-memory key transparently. Cost should be O(1) after
// first call (typical implementation: read from in-memory cache).
type KeyProvider func() (string, error)

// Logger is a minimal logging interface for diagnostic output. Apps may
// inject a logger (e.g. logrus, zap) via SetLogger; if none is set,
// internal recoveries are silent.
type Logger interface {
	Errorf(format string, args ...interface{})
}

var (
	keyProvider KeyProvider
	logger      Logger
)

// SetKeyProvider registers the callback used by StringEncrypt and
// StringDecrypt to resolve the master AES key. Typically called once
// during application init() after the underlying storage (file, DB) is
// available. Passing nil disables the convenience helpers — callers
// must then use StringEncryptWithKey / StringDecryptWithKey explicitly.
func SetKeyProvider(p KeyProvider) { keyProvider = p }

// SetLogger registers an optional logger used to record decrypt panics.
// Without one, those panics are recovered silently.
func SetLogger(l Logger) { logger = l }

func resolveKey() (string, error) {
	if keyProvider == nil {
		return "", errors.New("libpanel/encrypt: no key provider registered (call SetKeyProvider during app init)")
	}
	return keyProvider()
}

// StringEncrypt encrypts text using the key returned by the registered
// KeyProvider, with AES-256-GCM. Empty input is passed through.
func StringEncrypt(text string) (string, error) {
	if len(text) == 0 {
		return "", nil
	}
	key, err := resolveKey()
	if err != nil {
		return "", err
	}
	return StringEncryptWithKey(text, key)
}

// StringDecrypt decrypts text using the registered KeyProvider's key,
// supporting both v2 (GCM) and legacy v1 (CBC) ciphertexts. Empty
// input is passed through.
func StringDecrypt(text string) (string, error) {
	if len(text) == 0 {
		return "", nil
	}
	key, err := resolveKey()
	if err != nil {
		return "", err
	}
	return StringDecryptWithKey(text, key)
}

// StringEncryptWithKey is the pure-function variant of StringEncrypt.
// Used by callers that already hold the key in hand (e.g. installer-time
// bootstrap, tests).
func StringEncryptWithKey(text, key string) (string, error) {
	if len(text) == 0 || len(key) == 0 {
		return "", nil
	}
	return aesGCMEncrypt(deriveKeyV2(key), []byte(text))
}

// StringDecryptWithKey is the pure-function variant of StringDecrypt.
// Handles both v2 (GCM, SHA-256(key) as 32-byte AES-256 key) and v1
// (CBC, key truncated/padded to 16 bytes for AES-128).
//
// The legacy path pads short keys with 'u' to 16 bytes to match agent's
// historical behavior — safe for keys already ≥16 bytes (just truncates).
func StringDecryptWithKey(text, key string) (string, error) {
	defer func() {
		if r := recover(); r != nil && logger != nil {
			logger.Errorf("libpanel/encrypt: panic in StringDecryptWithKey: %v", r)
		}
	}()
	if len(text) == 0 {
		return "", nil
	}
	if strings.HasPrefix(text, v2Prefix) {
		return aesGCMDecrypt(deriveKeyV2(key), text[len(v2Prefix):])
	}
	legacyKey := key
	if len(legacyKey) < 16 {
		for len(legacyKey) < 16 {
			legacyKey += "u"
		}
	} else if len(legacyKey) > 16 {
		legacyKey = legacyKey[:16]
	}
	bytesPass, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return "", err
	}
	tpass, err := aesDecryptWithSalt([]byte(legacyKey), bytesPass)
	if err != nil {
		return "", err
	}
	return string(tpass), nil
}

// StringDecryptWithBase64 decrypts then base64-encodes the result.
// Useful for returning binary plaintext over JSON APIs.
func StringDecryptWithBase64(text string) (string, error) {
	decryptItem, err := StringDecrypt(text)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString([]byte(decryptItem)), nil
}

// StringEncryptWithBase64 base64-decodes the input then encrypts. Used
// when input was hex/base64 binary that should be stored encrypted.
func StringEncryptWithBase64(text string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return "", err
	}
	return StringEncrypt(string(raw))
}

// --- low-level crypto primitives ---

func deriveKeyV2(k string) []byte {
	sum := sha256.Sum256([]byte(k))
	return sum[:]
}

func aesGCMEncrypt(key []byte, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := aead.Seal(nil, nonce, plaintext, nil)
	out := append(nonce, ct...)
	return v2Prefix + base64.StdEncoding.EncodeToString(out), nil
}

func aesGCMDecrypt(key []byte, encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < aead.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, ct := raw[:aead.NonceSize()], raw[aead.NonceSize():]
	pt, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

func padding(plaintext []byte, blockSize int) []byte {
	padding := blockSize - len(plaintext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(plaintext, padtext...)
}

func unPadding(origData []byte) ([]byte, error) {
	length := len(origData)
	if length == 0 {
		return nil, errors.New("invalid padding size")
	}
	unpadding := int(origData[length-1])
	if unpadding == 0 || unpadding > length || unpadding > aes.BlockSize {
		return nil, errors.New("invalid padding")
	}
	for i := 0; i < unpadding; i++ {
		if origData[length-1-i] != byte(unpadding) {
			return nil, errors.New("invalid padding")
		}
	}
	return origData[:(length - unpadding)], nil
}

func aesEncryptWithSalt(key, plaintext []byte) ([]byte, error) {
	plaintext = padding(plaintext, aes.BlockSize)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[0:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	cbc := cipher.NewCBCEncrypter(block, iv)
	cbc.CryptBlocks(ciphertext[aes.BlockSize:], plaintext)
	return ciphertext, nil
}

func aesDecryptWithSalt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	if (len(ciphertext)-aes.BlockSize)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of the block size")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	cbc := cipher.NewCBCDecrypter(block, iv)
	cbc.CryptBlocks(ciphertext, ciphertext)
	return unPadding(ciphertext)
}

// --- RSA helpers (used by login flows that RSA-wrap an AES key) ---

func ParseRSAPrivateKey(privateKeyPEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, errors.New("failed to decode PEM block containing the private key")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

func aesDecrypt(ciphertext, key, iv []byte) ([]byte, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, errors.New("invalid AES key length: must be 16, 24, or 32 bytes")
	}
	if len(iv) != aes.BlockSize {
		return nil, errors.New("invalid IV length: must be 16 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)
	unpadded, err := pkcs7Unpad(ciphertext)
	if err != nil {
		return nil, err
	}
	return unpadded, nil
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, errors.New("invalid padding size")
	}
	padLength := int(data[length-1])
	if padLength == 0 || padLength > length {
		return nil, errors.New("invalid padding")
	}
	for i := 0; i < padLength; i++ {
		if data[length-1-i] != byte(padLength) {
			return nil, errors.New("invalid padding")
		}
	}
	return data[:length-padLength], nil
}

// DecryptPassword unwraps an "envelope" ciphertext of the form
// base64(rsa-wrapped-aes-key) ":" base64(iv) ":" base64(aes-cbc-ciphertext).
// Used by 1Panel's login flow where the browser RSA-encrypts the AES
// session key under the panel's public key.
func DecryptPassword(encryptedData string, privateKey *rsa.PrivateKey) (string, error) {
	parts := strings.Split(encryptedData, ":")
	if len(parts) != 3 {
		return "", errors.New("encrypted data format error")
	}
	keyCipher := parts[0]
	ivBase64 := parts[1]
	ciphertextBase64 := parts[2]

	encryptedAESKey, err := base64.StdEncoding.DecodeString(keyCipher)
	if err != nil {
		return "", errors.New("failed to decode keyCipher")
	}
	aesKey, err := rsa.DecryptPKCS1v15(rand.Reader, privateKey, encryptedAESKey)
	if err != nil {
		return "", errors.New("failed to decode AES Key")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return "", errors.New("failed to decrypt the encrypted data")
	}
	iv, err := base64.StdEncoding.DecodeString(ivBase64)
	if err != nil {
		return "", errors.New("failed to decode the IV")
	}
	password, err := aesDecrypt(ciphertext, aesKey, iv)
	if err != nil {
		return "", err
	}
	return string(password), nil
}

func ExportPrivateKeyToPEM(privateKey *rsa.PrivateKey) string {
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})
	return string(privateKeyPEM)
}

func ExportPublicKeyToPEM(publicKey *rsa.PublicKey) (string, error) {
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})
	return string(publicKeyPEM), nil
}
