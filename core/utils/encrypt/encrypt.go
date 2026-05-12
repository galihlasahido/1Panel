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

	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/global"
)

// v2Prefix marks ciphertext encrypted with AES-256-GCM (authenticated).
// Values without this prefix are legacy AES-128-CBC + PKCS7 from earlier
// versions and are still decrypted for backward compatibility.
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

func StringDecryptWithBase64(text string) (string, error) {
	decryptItem, err := StringDecrypt(text)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString([]byte(decryptItem)), nil
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
//  1. /etc/1panel/.encrypt_key (mode 0600) — preferred, lives outside the DB
//  2. global.CONF.Base.EncryptKey (process-cached value)
//  3. setting table EncryptKey row
//
// After loading from the DB or process cache, the key is best-effort persisted
// to the file so future boots prefer the file. This makes a stolen DB
// snapshot insufficient to decrypt secrets at rest.
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

func StringEncryptWithKey(text, key string) (string, error) {
	if len(text) == 0 || len(key) == 0 {
		return "", nil
	}
	return aesGCMEncrypt(deriveKeyV2(key), []byte(text))
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

func StringDecryptWithKey(text, key string) (string, error) {
	defer func() {
		if r := recover(); r != nil {
			if global.LOG != nil {
				global.LOG.Errorf("A panic occurred during string decrypt with key, error message: %v", r)
			}
		}
	}()
	if len(text) == 0 {
		return "", nil
	}
	if strings.HasPrefix(text, v2Prefix) {
		return aesGCMDecrypt(deriveKeyV2(key), text[len(v2Prefix):])
	}
	bytesPass, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return "", err
	}
	tpass, err := aesDecryptWithSalt([]byte(key), bytesPass)
	if err != nil {
		return "", err
	}
	return string(tpass), nil
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
	var block cipher.Block
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("iciphertext too short")
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
