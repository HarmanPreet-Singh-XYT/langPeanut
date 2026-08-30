// Package auth provides AES-256-GCM encryption for BYO API credentials at rest,
// and GitHub OAuth session helpers for the web UI login flow.
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// ─── Credential Encryption (AES-256-GCM) ─────────────────────────────────────
//
// The master key is a 32-byte hex string stored in the MASTER_KEY env var.
// It is never written to the DB; only the ciphertext (nonce prepended) is stored.

// Encrypt encrypts plaintext using AES-256-GCM.  Returns (nonce || ciphertext).
// masterKeyHex must be a 64-char hex string (32 bytes).
func Encrypt(masterKeyHex, plaintext string) ([]byte, error) {
	key, err := decodeKey(masterKeyHex)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return ciphertext, nil
}

// Decrypt decrypts a (nonce || ciphertext) blob produced by Encrypt.
func Decrypt(masterKeyHex string, blob []byte) (string, error) {
	key, err := decodeKey(masterKeyHex)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}
	ns := gcm.NonceSize()
	if len(blob) < ns {
		return "", errors.New("ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, blob[:ns], blob[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("gcm decrypt: %w", err)
	}
	return string(plaintext), nil
}

func decodeKey(hexKey string) ([]byte, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("decode master key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes (64 hex chars), got %d bytes", len(key))
	}
	return key, nil
}

// GenerateMasterKey generates a random 32-byte key encoded as hex.
// Useful for first-time setup: `langpeanut-cloud keygen`.
func GenerateMasterKey() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
