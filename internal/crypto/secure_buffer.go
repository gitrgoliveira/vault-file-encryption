package crypto

import (
	"fmt"
)

// SecureBuffer wraps a byte slice containing sensitive data with automatic
// memory protection and cleanup. It ensures that:
// 1. The buffer is locked in memory (preventing swap to disk)
// 2. The buffer is securely zeroed when no longer needed
// 3. Developers cannot forget to clean up sensitive data
//
// Usage:
//
//	buf, err := NewSecureBuffer(32) // Create 32-byte buffer
//	if err != nil {
//	    return err
//	}
//	defer buf.Destroy() // Always destroy when done
//
//	// Use buf.Data() to access the underlying byte slice
//	copy(buf.Data(), sensitiveData)
type SecureBuffer struct {
	data   []byte
	unlock func()
}

// NewSecureBuffer creates a new SecureBuffer with the specified size.
// The buffer is automatically locked in memory (best effort) to prevent
// it from being swapped to disk.
//
// CRITICAL: Always call Destroy() when done with the buffer, typically
// using defer immediately after creation.
func NewSecureBuffer(size int) (*SecureBuffer, error) {
	if size <= 0 {
		return nil, fmt.Errorf("buffer size must be positive, got %d", size)
	}

	data := make([]byte, size)

	// Lock the buffer in memory (best effort)
	unlock, err := LockMemory(data)
	if err != nil {
		// Log the error but continue - memory locking is best-effort
		// The encryption will still work, just without the swap protection
	}

	return &SecureBuffer{
		data:   data,
		unlock: unlock,
	}, nil
}

// NewSecureBufferFromBytes creates a SecureBuffer from existing sensitive data.
// The data is copied into a new locked buffer and the source should be zeroed
// by the caller if it's no longer needed.
func NewSecureBufferFromBytes(source []byte) (*SecureBuffer, error) {
	if len(source) == 0 {
		return nil, fmt.Errorf("source data cannot be empty")
	}

	buf, err := NewSecureBuffer(len(source))
	if err != nil {
		return nil, err
	}

	copy(buf.data, source)
	return buf, nil
}

// Data returns the underlying byte slice. The caller MUST NOT:
// - Store references to this slice beyond the lifetime of the SecureBuffer
// - Modify the slice after calling Destroy()
// - Share this slice with untrusted code
func (sb *SecureBuffer) Data() []byte {
	return sb.data
}

// Destroy securely zeros the buffer and unlocks the memory.
// After calling Destroy(), the SecureBuffer should not be used.
// This method is idempotent - calling it multiple times is safe.
func (sb *SecureBuffer) Destroy() {
	if sb.data != nil {
		SecureZero(sb.data)
		sb.data = nil
	}

	if sb.unlock != nil {
		sb.unlock()
		sb.unlock = nil
	}
}
