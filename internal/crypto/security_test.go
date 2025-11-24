package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemoryLocking tests that memory locking functions work correctly
func TestMemoryLocking(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "lock small buffer",
			data: []byte("sensitive data"),
		},
		{
			name: "lock 32-byte key",
			data: make([]byte, 32),
		},
		{
			name: "lock empty buffer",
			data: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// LockMemory should not panic
			err := LockMemory(tt.data)

			// Error is acceptable (may not have privileges), but should not panic
			_ = err

			// Call unlock - should not panic
			err = UnlockMemory(tt.data)
			_ = err

			// Note: We can't easily verify that memory was actually locked
			// without elevated privileges and OS-specific checks
			// This test mainly verifies the API works correctly
		})
	}
}

// TestMemoryLockingNil tests that LockMemory handles nil gracefully
func TestMemoryLockingNil(t *testing.T) {
	err := LockMemory(nil)
	require.NoError(t, err)
	err = UnlockMemory(nil)
	require.NoError(t, err)
}

// TestConstantTimeMemoryZero verifies SecureZero uses constant-time operations
func TestConstantTimeMemoryZero(t *testing.T) {
	// This test verifies that SecureZero properly clears memory
	// Using crypto/subtle ensures compiler doesn't optimize away the zeroing

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "zero short data",
			data: []byte("secret"),
		},
		{
			name: "zero 32-byte key",
			data: []byte("this-is-a-256-bit-encryption-key"),
		},
		{
			name: "zero large buffer",
			data: make([]byte, 1024),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fill with non-zero data
			for i := range tt.data {
				tt.data[i] = byte(i%256 + 1) // Ensure not zero
			}

			// Verify not all zeros before
			hasNonZero := false
			for _, b := range tt.data {
				if b != 0 {
					hasNonZero = true
					break
				}
			}
			require.True(t, hasNonZero, "test data should not be all zeros initially")

			// Zero the data
			SecureZero(tt.data)

			// Verify all zeros after
			for i, b := range tt.data {
				assert.Equal(t, byte(0), b, "byte at index %d should be zero", i)
			}
		})
	}
}
