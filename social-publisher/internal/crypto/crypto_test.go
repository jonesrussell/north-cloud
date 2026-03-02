package crypto_test

import (
	"encoding/hex"
	"testing"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	keyHex := hex.EncodeToString(key)

	plaintext := []byte(`{"api_key":"sk-test","api_secret":"secret123"}`)

	encrypted, err := crypto.Encrypt(plaintext, keyHex)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, encrypted)

	decrypted, err := crypto.Decrypt(encrypted, keyHex)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncrypt_DifferentNonceEachTime(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	keyHex := hex.EncodeToString(key)

	plaintext := []byte("same data")

	enc1, err := crypto.Encrypt(plaintext, keyHex)
	require.NoError(t, err)

	enc2, err := crypto.Encrypt(plaintext, keyHex)
	require.NoError(t, err)

	assert.NotEqual(t, enc1, enc2, "each encryption should produce different ciphertext due to random nonce")
}

func TestDecrypt_InvalidKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 0xFF
	keyHex1 := hex.EncodeToString(key1)
	keyHex2 := hex.EncodeToString(key2)

	encrypted, err := crypto.Encrypt([]byte("secret"), keyHex1)
	require.NoError(t, err)

	_, err = crypto.Decrypt(encrypted, keyHex2)
	assert.Error(t, err)
}

func TestEncrypt_InvalidKeyLength(t *testing.T) {
	_, err := crypto.Encrypt([]byte("data"), "tooshort")
	assert.Error(t, err)
}
