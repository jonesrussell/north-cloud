package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

const requiredKeyLen = 32

// Encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// The keyHex parameter must be a 64-character hex string (32 bytes).
// Returns nonce || ciphertext.
func Encrypt(plaintext []byte, keyHex string) ([]byte, error) {
	key, err := decodeKey(keyHex)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, readErr := io.ReadFull(rand.Reader, nonce); readErr != nil {
		return nil, fmt.Errorf("generating nonce: %w", readErr)
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts ciphertext produced by Encrypt.
// The keyHex parameter must be the same key used during encryption.
func Decrypt(ciphertext []byte, keyHex string) ([]byte, error) {
	key, err := decodeKey(keyHex)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, encrypted := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}

	return plaintext, nil
}

func decodeKey(keyHex string) ([]byte, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("decoding key: %w", err)
	}
	if len(key) != requiredKeyLen {
		return nil, fmt.Errorf("key must be %d bytes, got %d", requiredKeyLen, len(key))
	}
	return key, nil
}
