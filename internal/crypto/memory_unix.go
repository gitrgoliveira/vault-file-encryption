//go:build !windows

package crypto

import (
	"fmt"
	"syscall"
)

// lockMemoryPlatform is the Unix implementation of memory locking.
// It uses mlock to prevent memory from being swapped to disk.
func lockMemoryPlatform(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	err := syscall.Mlock(data)
	if err != nil {
		return fmt.Errorf("mlock failed: %w", err)
	}

	return nil
}

// unlockMemoryPlatform is the Unix implementation of memory unlocking.
func unlockMemoryPlatform(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	err := syscall.Munlock(data)
	if err != nil {
		return fmt.Errorf("munlock failed: %w", err)
	}

	return nil
}
