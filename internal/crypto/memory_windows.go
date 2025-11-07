//go:build windows

package crypto

// lockMemoryPlatform is the Windows implementation of memory locking.
// Windows does not provide a direct equivalent to Unix's mlock, so this
// is a no-op. The VirtualLock API exists but has different semantics and
// requires different handling.
func lockMemoryPlatform(data []byte) error {
	// On Windows, memory locking is not supported in the same way as Unix.
	// Return nil to allow the code to continue without this protection.
	return nil
}

// unlockMemoryPlatform is the Windows implementation of memory unlocking.
func unlockMemoryPlatform(data []byte) error {
	// No-op on Windows
	return nil
}
