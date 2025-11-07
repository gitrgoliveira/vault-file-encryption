package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSecureBuffer(t *testing.T) {
	tests := []struct {
		name      string
		size      int
		expectErr bool
	}{
		{
			name:      "valid size",
			size:      32,
			expectErr: false,
		},
		{
			name:      "zero size",
			size:      0,
			expectErr: true,
		},
		{
			name:      "negative size",
			size:      -1,
			expectErr: true,
		},
		{
			name:      "large size",
			size:      1024 * 1024,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, err := NewSecureBuffer(tt.size)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, buf)
			} else {
				require.NoError(t, err)
				require.NotNil(t, buf)
				defer buf.Destroy()

				assert.NotNil(t, buf.Data())
				assert.Equal(t, tt.size, len(buf.Data()))
			}
		})
	}
}

func TestNewSecureBufferFromBytes(t *testing.T) {
	tests := []struct {
		name      string
		source    []byte
		expectErr bool
	}{
		{
			name:      "valid data",
			source:    []byte("sensitive data"),
			expectErr: false,
		},
		{
			name:      "empty data",
			source:    []byte{},
			expectErr: true,
		},
		{
			name:      "nil data",
			source:    nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, err := NewSecureBufferFromBytes(tt.source)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, buf)
			} else {
				require.NoError(t, err)
				require.NotNil(t, buf)
				defer buf.Destroy()

				assert.Equal(t, len(tt.source), len(buf.Data()))
				assert.Equal(t, tt.source, buf.Data())
			}
		})
	}
}

func TestSecureBuffer_Destroy(t *testing.T) {
	buf, err := NewSecureBuffer(32)
	require.NoError(t, err)
	require.NotNil(t, buf)

	// Fill with data
	testData := []byte("sensitive key data here!!!!!!!!!")
	copy(buf.Data(), testData)

	// Verify data is there
	assert.Equal(t, testData, buf.Data())

	// Destroy
	buf.Destroy()

	// Verify data is zeroed
	assert.Nil(t, buf.Data())

	// Multiple destroys should be safe (idempotent)
	buf.Destroy()
	buf.Destroy()
}

func TestSecureBuffer_DataAccess(t *testing.T) {
	buf, err := NewSecureBuffer(16)
	require.NoError(t, err)
	require.NotNil(t, buf)
	defer buf.Destroy()

	// Write data
	testData := []byte("0123456789ABCDEF")
	copy(buf.Data(), testData)

	// Read data back
	assert.Equal(t, testData, buf.Data())

	// Verify length is correct
	assert.Equal(t, 16, len(buf.Data()))
}

func TestSecureBuffer_Integration(t *testing.T) {
	// Simulate typical usage pattern
	buf, err := NewSecureBuffer(32)
	require.NoError(t, err)
	defer buf.Destroy()

	// Simulate writing a key
	key := []byte("this-is-a-secret-encryption-key!")
	copy(buf.Data(), key)

	// Simulate using the key
	result := make([]byte, 32)
	copy(result, buf.Data())
	assert.Equal(t, key, result)

	// Destroy - data should be zeroed
	buf.Destroy()

	// Verify buffer is cleared
	assert.Nil(t, buf.Data())
}

func TestSecureBuffer_MemoryProtection(t *testing.T) {
	// This test verifies that memory locking doesn't cause errors
	// The actual locking may fail (due to permissions) but should not error out
	buf, err := NewSecureBuffer(1024)
	require.NoError(t, err)
	require.NotNil(t, buf)
	defer buf.Destroy()

	// Fill with data
	bufData := buf.Data()
	for i := 0; i < len(bufData); i++ {
		bufData[i] = byte(i % 256)
	}

	// Verify data is intact
	for i := 0; i < len(bufData); i++ {
		assert.Equal(t, byte(i%256), bufData[i])
	}
}
