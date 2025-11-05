package crypto

import "fmt"

// EncryptionError represents an encryption error with context
type EncryptionError struct {
	Op       string // Operation: "encrypt", "decrypt", "generate_key", etc.
	Path     string // File path being operated on
	ChunkNum int    // Chunk number if applicable (-1 if not chunked operation)
	Err      error  // Underlying error
}

func (e *EncryptionError) Error() string {
	if e.ChunkNum >= 0 {
		return fmt.Sprintf("%s %s (chunk %d): %v", e.Op, e.Path, e.ChunkNum, e.Err)
	}
	return fmt.Sprintf("%s %s: %v", e.Op, e.Path, e.Err)
}

func (e *EncryptionError) Unwrap() error {
	return e.Err
}

// NewEncryptionError creates a new EncryptionError
func NewEncryptionError(op, path string, err error) *EncryptionError {
	return &EncryptionError{
		Op:       op,
		Path:     path,
		ChunkNum: -1,
		Err:      err,
	}
}

// NewChunkEncryptionError creates a new EncryptionError for a chunk operation
func NewChunkEncryptionError(op, path string, chunkNum int, err error) *EncryptionError {
	return &EncryptionError{
		Op:       op,
		Path:     path,
		ChunkNum: chunkNum,
		Err:      err,
	}
}
