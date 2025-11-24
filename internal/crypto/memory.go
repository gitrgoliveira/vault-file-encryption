package crypto

import (
	"github.com/gitrgoliveira/go-fileencrypt/secure"
)

// SecureZero overwrites a byte slice with zeros to remove sensitive data from memory.
// Uses crypto/subtle to prevent compiler optimization from eliminating the zeroing operation.
func SecureZero(data []byte) {
	secure.Zero(data)
}

// LockMemory locks the given byte slice in memory to prevent swapping
func LockMemory(data []byte) error {
	return secure.LockMemory(data)
}

// UnlockMemory unlocks the given byte slice from memory
func UnlockMemory(data []byte) error {
	return secure.UnlockMemory(data)
}
