package crypto

import (
	"runtime"
)

// SecureZero overwrites a byte slice with zeros to remove sensitive data from memory
func SecureZero(data []byte) {
	if data == nil {
		return
	}

	// Overwrite all bytes with zero
	for i := range data {
		data[i] = 0
	}

	// Force garbage collection to ensure memory is cleared
	// Note: This is a hint to the GC, not a guarantee
	runtime.GC()
}
