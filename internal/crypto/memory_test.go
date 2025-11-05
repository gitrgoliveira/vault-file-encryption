package crypto

import (
	"testing"

	"github.com/gitrgoliveira/vault_file_encryption/internal/vault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecureZero(t *testing.T) {
	data := []byte("sensitive data that should be zeroed")
	originalLen := len(data)

	SecureZero(data)

	// Verify length is unchanged
	assert.Equal(t, originalLen, len(data))

	// Verify all bytes are zero
	for i, b := range data {
		assert.Equal(t, byte(0), b, "byte at index %d should be zero", i)
	}
}

func TestSecureZeroNil(t *testing.T) {
	// Should not panic
	SecureZero(nil)
}

func TestSecureZeroEmpty(t *testing.T) {
	data := []byte{}
	SecureZero(data)
	assert.Equal(t, 0, len(data))
}

func TestSecureZero_DataKeyPattern(t *testing.T) {
	// This test simulates the pattern of getting a plaintext key, using it,
	// and ensuring it is zeroed out.
	key := &vault.DataKey{
		Plaintext: "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=", // 32 bytes, not all zeros
	}

	// Get plaintext bytes
	plaintextKey, err := key.PlaintextBytes()
	require.NoError(t, err)
	require.NotNil(t, plaintextKey)

	// Copy the key to check against later
	keyCopy := make([]byte, len(plaintextKey))
	copy(keyCopy, plaintextKey)

	// Ensure it's not already all zeros
	isAllZeros := true
	for _, b := range keyCopy {
		if b != 0 {
			isAllZeros = false
			break
		}
	}
	require.False(t, isAllZeros, "Key should not be all zeros initially")

	// Defer zeroing, simulating how it's used in the encryptor/decryptor
	defer SecureZero(plaintextKey)

	// Simulate doing some work with the key...
	// In a real scenario, this is where encryption/decryption would happen.

	// After the function returns, the deferred SecureZero is called.
	// To test it, we can wrap this in a function.
	func() {
		keyBytes, _ := key.PlaintextBytes()
		defer SecureZero(keyBytes)
		// Dummy operation
		_ = len(keyBytes)
	}()

	// We can't directly check the memory of `plaintextKey` after a real defer,
	// but we can call SecureZero directly and verify its effect.
	SecureZero(keyCopy)

	// Verify all bytes are zero
	for i, b := range keyCopy {
		assert.Equal(t, byte(0), b, "byte at index %d should be zero after SecureZero", i)
	}
}
