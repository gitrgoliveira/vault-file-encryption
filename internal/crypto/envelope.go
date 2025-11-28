package crypto

import (
	"context"
	"fmt"
	"os"

	fileencrypt "github.com/gitrgoliveira/go-fileencrypt"
	"github.com/gitrgoliveira/vault-file-encryption/internal/vault"
)

// VaultClient interface defines the methods needed from vault client
type VaultClient interface {
	GenerateDataKey() (*vault.DataKey, error)
	DecryptDataKey(ciphertext string) (*vault.DataKey, error)
}

const (
	// DefaultChunkSize for file operations: 1MB
	DefaultChunkSize = 1024 * 1024

	// ProgressReportInterval is the percentage interval for progress logging
	ProgressReportInterval = 20.0
)

// EncryptorConfig holds configuration for the Encryptor
type EncryptorConfig struct {
	ChunkSize int // Chunk size in bytes
}

// Encryptor handles file encryption using envelope encryption
type Encryptor struct {
	vaultClient VaultClient
	config      *EncryptorConfig
}

// NewEncryptor creates a new Encryptor with the given configuration
func NewEncryptor(vaultClient VaultClient, cfg *EncryptorConfig) *Encryptor {
	if cfg == nil || cfg.ChunkSize == 0 {
		cfg = &EncryptorConfig{ChunkSize: DefaultChunkSize}
	}

	return &Encryptor{
		vaultClient: vaultClient,
		config:      cfg,
	}
}

// EncryptFile encrypts a file using envelope encryption and returns the encrypted data key
func (e *Encryptor) EncryptFile(ctx context.Context, sourcePath, destPath string, progressCallback func(float64)) (string, error) {
	// Generate a new data encryption key from Vault
	dataKey, err := e.vaultClient.GenerateDataKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate data key: %w", err)
	}
	// Ensure the plaintext key is wiped from memory when we're done
	defer dataKey.Destroy()

	// Encrypt the file using the plaintext key
	var opts []fileencrypt.Option
	if e.config.ChunkSize != 0 {
		opt, err := fileencrypt.WithChunkSize(e.config.ChunkSize)
		if err != nil {
			return "", fmt.Errorf("invalid chunk size: %w", err)
		}
		opts = append(opts, opt)
	}
	if progressCallback != nil {
		opts = append(opts, fileencrypt.WithProgress(progressCallback))
	}

	if err := fileencrypt.EncryptFile(ctx, sourcePath, destPath, dataKey.Plaintext, opts...); err != nil {
		return "", fmt.Errorf("failed to encrypt file: %w", err)
	}

	// Return the encrypted data key
	return dataKey.Ciphertext, nil
}

// Decryptor handles file decryption using envelope encryption
type Decryptor struct {
	vaultClient VaultClient
	config      *EncryptorConfig // Reuse EncryptorConfig
}

// NewDecryptor creates a new Decryptor with the given configuration
func NewDecryptor(vaultClient VaultClient, cfg *EncryptorConfig) *Decryptor {
	if cfg == nil || cfg.ChunkSize == 0 {
		cfg = &EncryptorConfig{ChunkSize: DefaultChunkSize}
	}

	return &Decryptor{
		vaultClient: vaultClient,
		config:      cfg,
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
	// Ensure the plaintext key is wiped from memory when we're done
	defer dataKey.Destroy()

	// Decrypt the file using the plaintext key
	var opts []fileencrypt.Option
	if d.config.ChunkSize != 0 {
		opt, err := fileencrypt.WithChunkSize(d.config.ChunkSize)
		if err != nil {
			return fmt.Errorf("invalid chunk size: %w", err)
		}
		opts = append(opts, opt)
	}
	if progressCallback != nil {
		opts = append(opts, fileencrypt.WithProgress(progressCallback))
	}

	if err := fileencrypt.DecryptFile(ctx, encryptedPath, destPath, dataKey.Plaintext, opts...); err != nil {
		return fmt.Errorf("failed to decrypt file: %w", err)
	}

	return nil
}
