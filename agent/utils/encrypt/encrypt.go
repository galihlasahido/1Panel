package encrypt

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/1Panel-dev/1Panel/agent/app/model"
	"github.com/1Panel-dev/1Panel/agent/global"
)

// v2Prefix marks ciphertext encrypted with AES-256-GCM (authenticated).
// Values without this prefix are legacy AES-128-CBC + PKCS7 and are still
// decrypted for backward compatibility.
const v2Prefix = "v2:"

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

func StringEncryptWithBase64(text string) (string, error) {
	accessKeyItem, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return "", err
	}
	encryptKeyItem, err := StringEncrypt(string(accessKeyItem))
	if err != nil {
		return "", err
	}
	return encryptKeyItem, nil
}

func StringEncryptWithKey(text, key string) (string, error) {
	if len(text) == 0 {
		return "", nil
	}
	if len(key) == 0 {
		return "", errors.New("empty encryption key")
	}
	return aesGCMEncrypt(deriveKeyV2(key), []byte(text))
}

func StringEncrypt(text string) (string, error) {
	if len(text) == 0 {
		return "", nil
	}
	key, err := resolveEncryptKey()
	if err != nil {
		return "", err
	}
	return StringEncryptWithKey(text, key)
}

// resolveEncryptKey returns the master AES key, sourcing it in priority order:
// file → process cache → setting row. See core/utils/encrypt/encrypt.go for
// the full rationale. After loading from a slower source the value is best
// effort written to the file so future boots use the file path.
func resolveEncryptKey() (string, error) {
	if k, ok := readKeyFile(); ok {
		if global.CONF.Base.EncryptKey != k {
			global.CONF.Base.EncryptKey = k
		}
		return k, nil
	}
	if len(global.CONF.Base.EncryptKey) > 0 {
		_ = writeKeyFile(global.CONF.Base.EncryptKey)
		return global.CONF.Base.EncryptKey, nil
	}
	var encryptSetting model.Setting
	if err := global.DB.Where("key = ?", "EncryptKey").First(&encryptSetting).Error; err != nil {
		return "", err
	}
	global.CONF.Base.EncryptKey = encryptSetting.Value
	_ = writeKeyFile(encryptSetting.Value)
	return encryptSetting.Value, nil
}

func StringDecryptWithBase64(text string) (string, error) {
	decryptItem, err := StringDecrypt(text)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString([]byte(decryptItem)), nil
}

func StringDecryptWithKey(text, key string) (string, error) {
	defer func() {
		if r := recover(); r != nil {
			global.LOG.Errorf("A panic occurred during string decrypt with key, error message: %v", r)
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
	} else {
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

func StringDecrypt(text string) (string, error) {
	if len(text) == 0 {
		return "", nil
	}
	key, err := resolveEncryptKey()
	if err != nil {
		return "", err
	}
	return StringDecryptWithKey(text, key)
}

func padding(plaintext []byte, blockSize int) []byte {
	padding := blockSize - len(plaintext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(plaintext, padtext...)
}

func unPadding(origData []byte) ([]byte, error) {
	length := len(origData)
	if length == 0 {
		return nil, fmt.Errorf("invalid padding size")
	}

	unpadding := int(origData[length-1])
	if unpadding == 0 || unpadding > length {
		return nil, fmt.Errorf("invalid padding")
	}

	for i := 0; i < unpadding; i++ {
		if origData[length-1-i] != byte(unpadding) {
			return nil, fmt.Errorf("invalid padding")
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
	var block cipher.Block
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("iciphertext too short")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	cbc := cipher.NewCBCDecrypter(block, iv)
	cbc.CryptBlocks(ciphertext, ciphertext)

	unpadded, err := unPadding(ciphertext)
	if err != nil {
		return nil, err
	}
	return unpadded, nil
}
