package rewrap

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// BackupOptions configures backup behavior.
type BackupOptions struct {
	Enabled bool   // Create backups
	Suffix  string // Backup file suffix
}

// BackupManager handles creation and restoration of key file backups.
type BackupManager struct {
	options BackupOptions
}

// NewBackupManager creates a new backup manager.
func NewBackupManager(options BackupOptions) *BackupManager {
	if options.Suffix == "" {
		options.Suffix = ".bak"
	}
	return &BackupManager{
		options: options,
	}
}

// CreateBackup creates a backup copy of a file atomically.
// Returns the backup file path if successful.
func (m *BackupManager) CreateBackup(filePath string) (string, error) {
	if !m.options.Enabled {
		return "", nil
	}

	// Verify source file exists
	srcInfo, err := os.Stat(filePath) // #nosec G304 - user-provided file path for backup
	if err != nil {
		return "", fmt.Errorf("source file not found: %w", err)
	}

	if srcInfo.IsDir() {
		return "", fmt.Errorf("cannot backup directory: %s", filePath)
	}

	// Generate backup path
	backupPath := filePath + m.options.Suffix

	// Create temporary file in the same directory for atomic operation
	tempFile, err := os.CreateTemp(filepath.Dir(filePath), ".backup-*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Ensure cleanup on error
	defer func() {
		if err != nil {
			_ = tempFile.Close()
			_ = os.Remove(tempPath)
		}
	}()

	// Open source file
	srcFile, err := os.Open(filePath) // #nosec G304 - user-provided file path for backup
	if err != nil {
		return "", fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() { _ = srcFile.Close() }()

	// Copy contents
	if _, err = io.Copy(tempFile, srcFile); err != nil {
		return "", fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Sync to disk
	if err = tempFile.Sync(); err != nil {
		return "", fmt.Errorf("failed to sync backup file: %w", err)
	}

	// Close temp file before rename
	if err = tempFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err = os.Rename(tempPath, backupPath); err != nil {
		return "", fmt.Errorf("failed to rename backup file: %w", err)
	}

	return backupPath, nil
}

// RestoreBackup restores a file from its backup.
func (m *BackupManager) RestoreBackup(originalPath string) error {
	backupPath := originalPath + m.options.Suffix

	// Verify backup exists
	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	// Create temporary file
	tempFile, err := os.CreateTemp(filepath.Dir(originalPath), ".restore-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Ensure cleanup on error
	defer func() {
		if err != nil {
			_ = tempFile.Close()
			_ = os.Remove(tempPath)
		}
	}()

	// Open backup file
	backupFile, err := os.Open(backupPath) // #nosec G304 - user-provided file path for backup restoration
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer func() { _ = backupFile.Close() }()

	// Copy contents
	if _, err = io.Copy(tempFile, backupFile); err != nil {
		return fmt.Errorf("failed to copy backup contents: %w", err)
	}

	// Sync to disk
	if err = tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync restored file: %w", err)
	}

	// Close temp file
	if err = tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err = os.Rename(tempPath, originalPath); err != nil {
		return fmt.Errorf("failed to rename restored file: %w", err)
	}

	return nil
}

// RemoveBackup deletes a backup file.
func (m *BackupManager) RemoveBackup(originalPath string) error {
	backupPath := originalPath + m.options.Suffix

	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove backup file: %w", err)
	}

	return nil
}

// BackupExists checks if a backup file exists for the given path.
func (m *BackupManager) BackupExists(originalPath string) bool {
	backupPath := originalPath + m.options.Suffix
	_, err := os.Stat(backupPath)
	return err == nil
}

// GetBackupPath returns the backup path for a given file.
func (m *BackupManager) GetBackupPath(originalPath string) string {
	return originalPath + m.options.Suffix
}
