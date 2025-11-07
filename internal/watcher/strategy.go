package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gitrgoliveira/vault-file-encryption/internal/crypto"
	"github.com/gitrgoliveira/vault-file-encryption/internal/logger"
	"github.com/gitrgoliveira/vault-file-encryption/internal/model"
)

// ProcessStrategy defines the interface for processing operations
type ProcessStrategy interface {
	Process(ctx context.Context, item *model.Item) error
}

// EncryptStrategy handles file encryption
type EncryptStrategy struct {
	encryptor         *crypto.Encryptor
	logger            logger.Logger
	calculateChecksum bool
}

// NewEncryptStrategy creates a new encryption strategy
func NewEncryptStrategy(enc *crypto.Encryptor, log logger.Logger, calculateChecksum bool) *EncryptStrategy {
	return &EncryptStrategy{
		encryptor:         enc,
		logger:            log,
		calculateChecksum: calculateChecksum,
	}
}

// Process encrypts a file
func (s *EncryptStrategy) Process(ctx context.Context, item *model.Item) error {
	// Calculate checksum if enabled
	if s.calculateChecksum {
		checksum, err := crypto.CalculateChecksum(item.SourcePath)
		if err != nil {
			return fmt.Errorf("failed to calculate checksum: %w", err)
		}
		item.Checksum = checksum

		// Save checksum in DESTINATION directory, named after ORIGINAL filename
		// Example: /source/data.txt -> /encrypted/data.txt.sha256
		// This keeps checksum with encrypted files, not with source
		originalName := filepath.Base(item.SourcePath)
		checksumPath := filepath.Join(filepath.Dir(item.DestPath), originalName+".sha256")
		if err := crypto.SaveChecksum(checksum, checksumPath); err != nil {
			return fmt.Errorf("failed to save checksum: %w", err)
		}
		item.ChecksumPath = checksumPath
	}

	// Progress callback
	progressCallback := func(progress float64) {
		if int(progress)%20 == 0 {
			s.logger.Info("Encryption progress",
				"id", item.ID,
				"file", filepath.Base(item.SourcePath),
				"progress", fmt.Sprintf("%.0f%%", progress),
			)
		}
	}

	// Encrypt file with context
	encryptedKey, err := s.encryptor.EncryptFile(
		ctx,
		item.SourcePath,
		item.DestPath,
		progressCallback,
	)
	if err != nil {
		return err
	}

	// Save encrypted key
	if err := os.WriteFile(item.KeyPath, []byte(encryptedKey), 0600); err != nil { // #nosec G306 - intentional key file write
		return fmt.Errorf("failed to save encrypted key: %w", err)
	}

	return nil
}

// DecryptStrategy handles file decryption
type DecryptStrategy struct {
	decryptor      *crypto.Decryptor
	logger         logger.Logger
	verifyChecksum bool
}

// NewDecryptStrategy creates a new decryption strategy
func NewDecryptStrategy(dec *crypto.Decryptor, log logger.Logger, verifyChecksum bool) *DecryptStrategy {
	return &DecryptStrategy{
		decryptor:      dec,
		logger:         log,
		verifyChecksum: verifyChecksum,
	}
}

// Process decrypts a file
func (s *DecryptStrategy) Process(ctx context.Context, item *model.Item) error {
	// Progress callback
	progressCallback := func(progress float64) {
		if int(progress)%20 == 0 {
			s.logger.Info("Decryption progress",
				"id", item.ID,
				"file", filepath.Base(item.SourcePath),
				"progress", fmt.Sprintf("%.0f%%", progress),
			)
		}
	}

	// Decrypt file with context
	err := s.decryptor.DecryptFile(
		ctx,
		item.SourcePath,
		item.KeyPath,
		item.DestPath,
		progressCallback,
	)
	if err != nil {
		return err
	}

	// Verify checksum if enabled
	if s.verifyChecksum {
		// Checksum file is based on the ORIGINAL source file that was encrypted
		// For decryption: item.SourcePath is data.txt.enc, original was data.txt
		// Remove .enc extension to get original filename, then add .sha256
		originalFile := filepath.Base(item.SourcePath)
		originalFile = originalFile[:len(originalFile)-4] // Remove ".enc"
		checksumPath := filepath.Join(filepath.Dir(item.SourcePath), originalFile+".sha256")

		if _, err := os.Stat(checksumPath); err == nil {
			expectedChecksum, err := crypto.LoadChecksum(checksumPath)
			if err != nil {
				s.logger.Error("Failed to load checksum for verification", "error", err)
			} else {
				valid, err := crypto.VerifyChecksum(item.DestPath, expectedChecksum)
				if err != nil {
					return fmt.Errorf("failed to verify checksum: %w", err)
				}
				if !valid {
					return fmt.Errorf("checksum verification failed")
				}
				s.logger.Info("Checksum verified", "file", item.DestPath, "checksum_file", checksumPath)
			}
		} else {
			s.logger.Info("Checksum file not found, skipping verification", "checksum_file", checksumPath)
		}
	}

	return nil
}
