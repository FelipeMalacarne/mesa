// Package crypto provides AES encryption and decryption functionalities.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

type AESManager struct {
	key []byte
}

func NewAESManager(key string) (*AESManager, error) {
	// A chave deve ter 32 bytes para AES-256
	if len(key) != 32 {
		return nil, errors.New("master key must be exactly 32 bytes")
	}
	return &AESManager{key: []byte(key)}, nil
}

func (m *AESManager) Encrypt(plainText string) (string, error) {
	block, err := aes.NewCipher(m.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	cipherText := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

func (m *AESManager) Decrypt(cipherTextBase64 string) (string, error) {
	cipherText, err := base64.StdEncoding.DecodeString(cipherTextBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	block, err := aes.NewCipher(m.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(cipherText) < nonceSize {
		return "", errors.New("cipherText is too short")
	}

	nonce, actualCipherText := cipherText[:nonceSize], cipherText[nonceSize:]

	plainText, err := gcm.Open(nil, nonce, actualCipherText, nil)
	if err != nil {
		return "", errors.New("integrity check failed: invalid key or tampered data")
	}

	return string(plainText), nil
}
