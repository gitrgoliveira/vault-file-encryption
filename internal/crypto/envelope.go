package crypto

import (
	"bufio"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/gitrgoliveira/vault-file-encryption/internal/vault"
)

// VaultClient interface defines the methods needed from vault client
type VaultClient interface {
	GenerateDataKey() (*vault.DataKey, error)
	DecryptDataKey(ciphertext string) (*vault.DataKey, error)
}

const (
	// DefaultChunkSize for reading files (1MB chunks)
	DefaultChunkSize = 1024 * 1024

	// GCMNonceSize is the size of the GCM nonce
	GCMNonceSize = 12

	// ProgressReportInterval is the percentage interval for progress logging
	ProgressReportInterval = 20.0

	// MaxChunksPerFile is the maximum number of chunks allowed per file
	// With 1MB chunks, this allows files up to ~4 petabytes (2^32 * 1MB)
	// This prevents nonce overflow (12-byte nonce = 96 bits = 2^96 possible values)
	MaxChunksPerFile = 1 << 32 // 4,294,967,296 chunks

	// MaxChunkSize is the maximum size allowed for a single chunk (10MB)
	// This prevents memory exhaustion attacks from malformed encrypted files
	MaxChunkSize = 10 * 1024 * 1024
)

// EncryptorConfig holds configuration for the Encryptor
type EncryptorConfig struct {
	ChunkSize int // Chunk size in bytes
}

// Encryptor handles file encryption using envelope encryption
type Encryptor struct {
	vaultClient VaultClient
	config      *EncryptorConfig
	bufferPool  *sync.Pool
}

// NewEncryptor creates a new Encryptor with the given configuration
func NewEncryptor(vaultClient VaultClient, cfg *EncryptorConfig) *Encryptor {
	if cfg == nil || cfg.ChunkSize == 0 {
		cfg = &EncryptorConfig{ChunkSize: DefaultChunkSize}
	}

	return &Encryptor{
		vaultClient: vaultClient,
		config:      cfg,
		bufferPool: &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, cfg.ChunkSize)
				return &buf
			},
		},
	}
}

// EncryptFile encrypts a file using envelope encryption
// Returns the encrypted data key (ciphertext) and error
func (e *Encryptor) EncryptFile(ctx context.Context, sourcePath, destPath string, progressCallback func(float64)) (string, error) {
	// Generate data encryption key from Vault
	dataKey, err := e.vaultClient.GenerateDataKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate data key: %w", err)
	}

	// Convert plaintext key from base64 to bytes
	plaintextKeyBytes, err := dataKey.PlaintextBytes()
	if err != nil {
		return "", fmt.Errorf("failed to decode data key: %w", err)
	}

	// Create secure buffer for the plaintext key
	// This automatically locks memory and will zero on destroy
	plaintextKey, err := NewSecureBufferFromBytes(plaintextKeyBytes)
	if err != nil {
		SecureZero(plaintextKeyBytes) // Clean up the temporary bytes
		return "", fmt.Errorf("failed to create secure buffer: %w", err)
	}
	defer plaintextKey.Destroy()

	// Zero the temporary bytes now that we have a secure buffer
	SecureZero(plaintextKeyBytes)

	// Encrypt the file with the plaintext DEK
	if err := e.encryptFileWithKey(ctx, sourcePath, destPath, plaintextKey.Data(), progressCallback); err != nil {
		return "", fmt.Errorf("failed to encrypt file: %w", err)
	}

	// Return the encrypted DEK (ciphertext) to be saved separately
	return dataKey.Ciphertext, nil
}

// encryptFileWithKey encrypts a file using AES-256-GCM with buffered I/O and context support
func (e *Encryptor) encryptFileWithKey(ctx context.Context, sourcePath, destPath string, key []byte, progressCallback func(float64)) error {
	// Open source file
	sourceFile, err := os.Open(sourcePath) // #nosec G304 - intentional file encryption tool // #nosec G304 - intentional file encryption tool
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() {
		if closeErr := sourceFile.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close source file: %w", closeErr)
		}
	}()

	// Get file size for progress tracking
	fileInfo, err := sourceFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	fileSize := fileInfo.Size()

	// Create buffered reader
	bufferedReader := bufio.NewReaderSize(sourceFile, e.config.ChunkSize)

	// Create destination file
	destFile, err := os.Create(destPath) // #nosec G304 - intentional file encryption tool
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		if closeErr := destFile.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close destination file: %w", closeErr)
		}
	}()

	// Create buffered writer
	bufferedWriter := bufio.NewWriterSize(destFile, e.config.ChunkSize)
	defer func() {
		if flushErr := bufferedWriter.Flush(); flushErr != nil && err == nil {
			err = fmt.Errorf("failed to flush buffer: %w", flushErr)
		}
	}()

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	// For large files, we'll encrypt in chunks
	// Each chunk gets its own nonce (incremented from base nonce)
	baseNonce := make([]byte, GCMNonceSize)
	if _, err := io.ReadFull(rand.Reader, baseNonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Write the base nonce to the file first
	if _, err := bufferedWriter.Write(baseNonce); err != nil {
		return fmt.Errorf("failed to write nonce: %w", err)
	}

	// Write file size (8 bytes, big-endian) for integrity checking
	fileSizeBytes := make([]byte, 8)
	fileSizeBytes[0] = byte(fileSize >> 56)
	fileSizeBytes[1] = byte(fileSize >> 48)
	fileSizeBytes[2] = byte(fileSize >> 40)
	fileSizeBytes[3] = byte(fileSize >> 32)
	fileSizeBytes[4] = byte(fileSize >> 24)
	fileSizeBytes[5] = byte(fileSize >> 16)
	fileSizeBytes[6] = byte(fileSize >> 8)
	fileSizeBytes[7] = byte(fileSize)

	if _, err := bufferedWriter.Write(fileSizeBytes); err != nil {
		return fmt.Errorf("failed to write file size: %w", err)
	}

	// Get buffer from pool
	bufPtr := e.bufferPool.Get().(*[]byte)
	buffer := *bufPtr
	defer e.bufferPool.Put(bufPtr)

	// Initialize chunk nonce (will be incremented for each chunk)
	chunkNonce := make([]byte, GCMNonceSize)
	copy(chunkNonce, baseNonce)

	// Process file in chunks
	var totalBytesRead int64
	chunkIndex := 0
	nextMilestone := ProgressReportInterval

	for {
		// Check for nonce overflow before processing chunk
		if chunkIndex >= MaxChunksPerFile {
			return fmt.Errorf("file too large: exceeds maximum chunk limit of %d", MaxChunksPerFile)
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read chunk from buffered reader
		n, err := bufferedReader.Read(buffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read file: %w", err)
		}
		if n == 0 {
			break
		}

		// Encrypt chunk with current nonce
		// Use file size as additional authenticated data for GCM
		additionalData := fileSizeBytes
		ciphertext := gcm.Seal(nil, chunkNonce, buffer[:n], additionalData) // #nosec G407 - unique nonce per chunk

		// Write encrypted chunk size (4 bytes) then encrypted data
		chunkSizeBytes := make([]byte, 4)
		chunkSizeBytes[0] = byte(len(ciphertext) >> 24)
		chunkSizeBytes[1] = byte(len(ciphertext) >> 16)
		chunkSizeBytes[2] = byte(len(ciphertext) >> 8)
		chunkSizeBytes[3] = byte(len(ciphertext))

		if _, err := bufferedWriter.Write(chunkSizeBytes); err != nil {
			return fmt.Errorf("failed to write chunk size: %w", err)
		}

		if _, err := bufferedWriter.Write(ciphertext); err != nil {
			return fmt.Errorf("failed to write encrypted chunk: %w", err)
		}

		// Update progress
		totalBytesRead += int64(n)
		chunkIndex++

		// Increment nonce for next chunk (more efficient than loop)
		incrementNonce(chunkNonce)

		if progressCallback != nil && fileSize > 0 {
			progress := float64(totalBytesRead) / float64(fileSize) * 100.0
			if progress >= nextMilestone {
				progressCallback(progress)
				nextMilestone += ProgressReportInterval
			}
		}
	}

	// Final progress callback
	if progressCallback != nil {
		progressCallback(100.0)
	}

	return nil
}

// Decryptor handles file decryption using envelope encryption
type Decryptor struct {
	vaultClient VaultClient
	config      *EncryptorConfig // Reuse EncryptorConfig
	bufferPool  *sync.Pool
}

// NewDecryptor creates a new Decryptor with the given configuration
func NewDecryptor(vaultClient VaultClient, cfg *EncryptorConfig) *Decryptor {
	if cfg == nil || cfg.ChunkSize == 0 {
		cfg = &EncryptorConfig{ChunkSize: DefaultChunkSize}
	}

	return &Decryptor{
		vaultClient: vaultClient,
		config:      cfg,
		bufferPool: &sync.Pool{
			New: func() interface{} {
				// Buffer size for encrypted chunks (slightly larger than ChunkSize for GCM overhead)
				buf := make([]byte, cfg.ChunkSize+1024)
				return &buf
			},
		},
	}
}

// DecryptFile decrypts a file using envelope encryption
func (d *Decryptor) DecryptFile(ctx context.Context, encryptedPath, keyPath, destPath string, progressCallback func(float64)) error {
	// Read encrypted data key from file
	encryptedKeyData, err := os.ReadFile(keyPath) // #nosec G304 - intentional file encryption tool
	if err != nil {
		return fmt.Errorf("failed to read key file: %w", err)
	}

	// Decrypt the data key using Vault
	dataKey, err := d.vaultClient.DecryptDataKey(string(encryptedKeyData))
	if err != nil {
		return fmt.Errorf("failed to decrypt data key: %w", err)
	}

	// Convert plaintext key from base64 to bytes
	plaintextKeyBytes, err := dataKey.PlaintextBytes()
	if err != nil {
		return fmt.Errorf("failed to decode data key: %w", err)
	}

	// Create secure buffer for the plaintext key
	// This automatically locks memory and will zero on destroy
	plaintextKey, err := NewSecureBufferFromBytes(plaintextKeyBytes)
	if err != nil {
		SecureZero(plaintextKeyBytes) // Clean up the temporary bytes
		return fmt.Errorf("failed to create secure buffer: %w", err)
	}
	defer plaintextKey.Destroy()

	// Zero the temporary bytes now that we have a secure buffer
	SecureZero(plaintextKeyBytes)

	// Decrypt the file with the plaintext DEK
	if err := d.decryptFileWithKey(ctx, encryptedPath, destPath, plaintextKey.Data(), progressCallback); err != nil {
		return fmt.Errorf("failed to decrypt file: %w", err)
	}

	return nil
}

// decryptFileWithKey decrypts a file using AES-256-GCM with buffered I/O and context support
func (d *Decryptor) decryptFileWithKey(ctx context.Context, sourcePath, destPath string, key []byte, progressCallback func(float64)) error {
	// Open encrypted file
	sourceFile, err := os.Open(sourcePath) // #nosec G304 - intentional file encryption tool
	if err != nil {
		return fmt.Errorf("failed to open encrypted file: %w", err)
	}
	defer func() {
		if closeErr := sourceFile.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close source file: %w", closeErr)
		}
	}()

	// Get file size for progress tracking
	fileInfo, err := sourceFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	fileSize := fileInfo.Size()

	// Create buffered reader
	bufferedReader := bufio.NewReaderSize(sourceFile, d.config.ChunkSize)

	// Create destination file
	destFile, err := os.Create(destPath) // #nosec G304 - intentional file encryption tool
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		if closeErr := destFile.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close destination file: %w", closeErr)
		}
	}()

	// Create buffered writer
	bufferedWriter := bufio.NewWriterSize(destFile, d.config.ChunkSize)
	defer func() {
		if flushErr := bufferedWriter.Flush(); flushErr != nil && err == nil {
			err = fmt.Errorf("failed to flush buffer: %w", flushErr)
		}
	}()

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	// Read the base nonce
	baseNonce := make([]byte, GCMNonceSize)
	if _, err := io.ReadFull(bufferedReader, baseNonce); err != nil {
		return fmt.Errorf("failed to read nonce: %w", err)
	}

	// Read the original file size (8 bytes)
	fileSizeBytes := make([]byte, 8)
	if _, err := io.ReadFull(bufferedReader, fileSizeBytes); err != nil {
		return fmt.Errorf("failed to read file size: %w", err)
	}

	// Get buffer from pool
	bufPtr := d.bufferPool.Get().(*[]byte)
	defer d.bufferPool.Put(bufPtr)

	// Initialize chunk nonce (will be incremented for each chunk)
	chunkNonce := make([]byte, GCMNonceSize)
	copy(chunkNonce, baseNonce)

	// Process file in chunks
	var totalBytesRead int64 = GCMNonceSize + 8 // nonce + file size
	chunkIndex := 0
	nextMilestone := ProgressReportInterval

	for {
		// Check for nonce overflow before processing chunk
		if chunkIndex >= MaxChunksPerFile {
			return fmt.Errorf("file too large: exceeds maximum chunk limit of %d", MaxChunksPerFile)
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read chunk size (4 bytes)
		chunkSizeBytes := make([]byte, 4)
		n, err := bufferedReader.Read(chunkSizeBytes)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read chunk size: %w", err)
		}
		if n != 4 {
			return fmt.Errorf("incomplete chunk size read")
		}

		// Parse chunk size
		chunkSize := int(chunkSizeBytes[0])<<24 | int(chunkSizeBytes[1])<<16 | int(chunkSizeBytes[2])<<8 | int(chunkSizeBytes[3])

		// Validate chunk size to prevent memory exhaustion attacks
		if chunkSize <= 0 || chunkSize > MaxChunkSize {
			return fmt.Errorf("invalid chunk size: %d (must be between 1 and %d bytes)", chunkSize, MaxChunkSize)
		}

		// Read encrypted chunk using buffer from pool
		encryptedChunk := (*bufPtr)[:chunkSize]
		if _, err := io.ReadFull(bufferedReader, encryptedChunk); err != nil {
			return fmt.Errorf("failed to read encrypted chunk: %w", err)
		}

		// Decrypt chunk with current nonce
		// Use file size as additional authenticated data for GCM
		additionalData := fileSizeBytes
		plaintext, err := gcm.Open(nil, chunkNonce, encryptedChunk, additionalData)
		if err != nil {
			return fmt.Errorf("failed to decrypt chunk: %w", err)
		}

		// Write decrypted data to buffered writer
		if _, err := bufferedWriter.Write(plaintext); err != nil {
			return fmt.Errorf("failed to write decrypted chunk: %w", err)
		}

		// Update progress
		totalBytesRead += int64(4 + chunkSize)
		chunkIndex++

		// Increment nonce for next chunk (more efficient than loop)
		incrementNonce(chunkNonce)

		if progressCallback != nil && fileSize > 0 {
			progress := float64(totalBytesRead) / float64(fileSize) * 100.0
			if progress >= nextMilestone {
				progressCallback(progress)
				nextMilestone += ProgressReportInterval
			}
		}
	}

	// Final progress callback
	if progressCallback != nil {
		progressCallback(100.0)
	}

	return nil
}

// incrementNonce increments a nonce by 1 (treating it as a big-endian integer)
func incrementNonce(nonce []byte) {
	for i := len(nonce) - 1; i >= 0; i-- {
		nonce[i]++
		if nonce[i] != 0 {
			break
		}
	}
}
