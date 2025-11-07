package crypto

import (
	"crypto/subtle"
	"runtime"
)

// SecureZero overwrites a byte slice with zeros to remove sensitive data from memory.
// Uses crypto/subtle to prevent compiler optimization from eliminating the zeroing operation.
func SecureZero(data []byte) {
	if len(data) == 0 {
		return
	}

	// Use subtle.ConstantTimeCopy to ensure the compiler doesn't optimize away the zeroing
	// This is more reliable than a simple loop which can be optimized out
	zeros := make([]byte, len(data))
	subtle.ConstantTimeCopy(1, data, zeros)

	// Force garbage collection to ensure memory is cleared
	// Note: This is a hint to the GC, not a guarantee
	runtime.GC()
}

// LockMemory locks a byte slice in memory to prevent it from being swapped to disk.
// This is a best-effort operation and may fail on some systems or require elevated privileges.
// Returns an unlock function that should be called when the memory is no longer needed.
//
// Platform support:
//   - Unix/Linux/macOS: Uses mlock to lock memory pages
//   - Windows: No-op (returns success but memory is not locked)
func LockMemory(data []byte) (unlock func(), err error) {
	if len(data) == 0 {
		return func() {}, nil
	}

	// Lock the memory using platform-specific implementation
	if err := lockMemoryPlatform(data); err != nil {
		// If locking fails (e.g., insufficient privileges), continue without locking
		// This is logged but not fatal as the encryption will still work
		return func() {}, err
	}

	// Return unlock function
	unlock = func() {
		_ = unlockMemoryPlatform(data)
	}

	return unlock, nil
}
