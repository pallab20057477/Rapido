package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
)

// EncryptionService provides AES-256-GCM encryption for PII
type EncryptionService struct {
	key []byte
}

// NewEncryptionService creates encryption service with key from env
func NewEncryptionService(key string) (*EncryptionService, error) {
	if key == "" {
		return nil, fmt.Errorf("encryption key not provided")
	}

	// Derive 32-byte key from provided key using SHA-256
	hash := sha256.Sum256([]byte(key))

	return &EncryptionService{
		key: hash[:],
	}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM
func (e *EncryptionService) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
func (e *EncryptionService) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// HashForIndex creates deterministic hash for searching
func (e *EncryptionService) HashForIndex(data string) string {
	if data == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// MaskData masks sensitive data for logging (generic)
func MaskData(data string, visiblePrefix int, visibleSuffix int) string {
	if len(data) <= visiblePrefix+visibleSuffix {
		return "***"
	}

	prefix := data[:visiblePrefix]
	suffix := data[len(data)-visibleSuffix:]
	return prefix + "***" + suffix
}

// Global encryption instance
var Encrypter *EncryptionService

// InitEncryption initializes global encryption service
func InitEncryption(key string) error {
	var err error
	Encrypter, err = NewEncryptionService(key)
	return err
}
