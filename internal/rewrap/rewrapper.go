package rewrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/gitrgoliveira/vault_file_encryption/internal/logger"
	"github.com/gitrgoliveira/vault_file_encryption/internal/vault"
)

// RewrapOptions configures the rewrap operation.
type RewrapOptions struct {
	VaultClient  *vault.Client // Vault client for rewrapping
	MinVersion   int           // Minimum key version to require
	DryRun       bool          // If true, don't modify files
	CreateBackup bool          // Whether to create backups
	BackupSuffix string        // Backup file suffix (default: ".bak")
	Logger       logger.Logger // Logger interface (not pointer)
}

// Rewrapper orchestrates the key re-wrapping process.
type Rewrapper struct {
	options       RewrapOptions
	backupManager *BackupManager
}

// NewRewrapper creates a new key re-wrapper.
func NewRewrapper(options RewrapOptions) (*Rewrapper, error) {
	if options.VaultClient == nil {
		return nil, fmt.Errorf("vault client is required")
	}

	if options.MinVersion < 1 {
		return nil, fmt.Errorf("min_version must be >= 1")
	}

	if options.Logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	// Create backup manager
	backupManager := NewBackupManager(BackupOptions{
		Enabled: options.CreateBackup,
		Suffix:  options.BackupSuffix,
	})

	return &Rewrapper{
		options:       options,
		backupManager: backupManager,
	}, nil
}

// RewrapFile processes a single .key file.
func (r *Rewrapper) RewrapFile(ctx context.Context, keyFilePath string) (*vault.RewrapResult, error) {
	result := &vault.RewrapResult{
		FilePath: keyFilePath,
	}

	// Read current key file
	ciphertext, err := os.ReadFile(keyFilePath) // #nosec G304 - user-provided key file path
	if err != nil {
		result.Error = fmt.Errorf("failed to read key file: %w", err)
		return result, result.Error
	}

	oldCiphertext := string(ciphertext)
	result.OldCiphertext = oldCiphertext

	// Get current version
	info, err := vault.GetKeyVersionInfo(keyFilePath, oldCiphertext, r.options.MinVersion)
	if err != nil {
		result.Error = fmt.Errorf("failed to get key version: %w", err)
		return result, result.Error
	}

	result.OldVersion = info.Version

	// Check if rewrap is needed
	if !info.NeedsRewrap {
		r.options.Logger.Info("file already at minimum version",
			"file", keyFilePath,
			"version", info.Version,
			"min_version", r.options.MinVersion)
		return result, nil
	}

	r.options.Logger.Info("rewrapping key file",
		"file", keyFilePath,
		"old_version", info.Version,
		"min_version", r.options.MinVersion)

	// Dry run - don't modify files
	if r.options.DryRun {
		r.options.Logger.Info("dry-run mode: skipping file modification", "file", keyFilePath)
		return result, nil
	}

	// Create backup if enabled
	if r.options.CreateBackup {
		backupPath, err := r.backupManager.CreateBackup(keyFilePath)
		if err != nil {
			result.Error = fmt.Errorf("failed to create backup: %w", err)
			return result, result.Error
		}
		result.BackupCreated = true
		r.options.Logger.Info("backup created", "file", keyFilePath, "backup", backupPath)
	}

	// Call Vault to rewrap the key
	newCiphertext, err := r.options.VaultClient.RewrapDataKey(ctx, oldCiphertext)
	if err != nil {
		result.Error = fmt.Errorf("vault rewrap failed: %w", err)

		// Restore backup if rewrap failed
		if r.options.CreateBackup {
			if restoreErr := r.backupManager.RestoreBackup(keyFilePath); restoreErr != nil {
				r.options.Logger.Error("failed to restore backup after rewrap failure",
					"file", keyFilePath,
					"rewrap_error", err,
					"restore_error", restoreErr)
			}
		}

		return result, result.Error
	}

	result.NewCiphertext = newCiphertext

	// Get new version
	newVersion, err := vault.GetKeyVersion(newCiphertext)
	if err != nil {
		result.Error = fmt.Errorf("failed to get new key version: %w", err)
		return result, result.Error
	}

	result.NewVersion = newVersion

	// Write new ciphertext atomically
	if err := r.writeKeyFileAtomic(keyFilePath, []byte(newCiphertext)); err != nil {
		result.Error = fmt.Errorf("failed to write new key file: %w", err)

		// Restore backup if write failed
		if r.options.CreateBackup {
			if restoreErr := r.backupManager.RestoreBackup(keyFilePath); restoreErr != nil {
				r.options.Logger.Error("failed to restore backup after write failure",
					"file", keyFilePath,
					"write_error", err,
					"restore_error", restoreErr)
			}
		}

		return result, result.Error
	}

	r.options.Logger.Info("rewrap successful",
		"file", keyFilePath,
		"old_version", result.OldVersion,
		"new_version", result.NewVersion)

	return result, nil
}

// RewrapBatch processes multiple key files.
func (r *Rewrapper) RewrapBatch(ctx context.Context, keyFiles []string) ([]*vault.RewrapResult, error) {
	results := make([]*vault.RewrapResult, 0, len(keyFiles))
	var mu sync.Mutex

	r.options.Logger.Info("starting batch rewrap",
		"total_files", len(keyFiles),
		"min_version", r.options.MinVersion,
		"dry_run", r.options.DryRun)

	for i, keyFile := range keyFiles {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		r.options.Logger.Info("processing file",
			"file", keyFile,
			"progress", fmt.Sprintf("%d/%d", i+1, len(keyFiles)))

		result, err := r.RewrapFile(ctx, keyFile)

		mu.Lock()
		results = append(results, result)
		mu.Unlock()

		if err != nil {
			r.options.Logger.Error("failed to rewrap file",
				"file", keyFile,
				"error", err)
		}
	}

	r.options.Logger.Info("batch rewrap complete",
		"total_files", len(keyFiles),
		"processed", len(results))

	return results, nil
}

// writeKeyFileAtomic writes a key file atomically using temp file + rename.
func (r *Rewrapper) writeKeyFileAtomic(filePath string, data []byte) error {
	// Create temp file in same directory
	tempFile, err := os.CreateTemp(filepath.Dir(filePath), ".rewrap-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Cleanup on error
	defer func() {
		if err != nil {
			_ = tempFile.Close()
			_ = os.Remove(tempPath)
		}
	}()

	// Write data
	if _, err = tempFile.Write(data); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Sync to disk
	if err = tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close before rename
	if err = tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err = os.Rename(tempPath, filePath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}
