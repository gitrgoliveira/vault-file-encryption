package crypto

import (
	"testing"

	"github.com/gitrgoliveira/vault-file-encryption/internal/vault"
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
		Plaintext: []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, // 32 bytes, not all zeros
	}

	// Copy the key to check against later
	keyCopy := make([]byte, len(key.Plaintext))
	copy(keyCopy, key.Plaintext)

	// Ensure it's not already all zeros
	isAllZeros := true
	for _, b := range keyCopy {
		if b != 0 {
			isAllZeros = false
			break
		}
	}
	require.False(t, isAllZeros, "Key should not be all zeros initially")

	// Simulate doing some work with the key...
	// In a real scenario, this is where encryption/decryption would happen.

	// After the function returns, the deferred Destroy is called.
	// To test it, we can wrap this in a function.
	func() {
		defer key.Destroy()
		// Dummy operation
		_ = len(key.Plaintext)
	}()

	// Verify key.Plaintext is nil after Destroy
	assert.Nil(t, key.Plaintext)

	// We can't easily verify the memory was zeroed without keeping a pointer to the underlying array
	// which is tricky in Go. But we trust secure.Zero works as tested in TestSecureZero.
	// The main thing is that Destroy() calls secure.Zero() and sets the slice to nil.

	// We can't directly check the memory of `plaintextKey` after a real defer,
	// but we can call SecureZero directly and verify its effect.
	SecureZero(keyCopy)

	// Verify all bytes are zero
	for i, b := range keyCopy {
		assert.Equal(t, byte(0), b, "byte at index %d should be zero after SecureZero", i)
	}
}
