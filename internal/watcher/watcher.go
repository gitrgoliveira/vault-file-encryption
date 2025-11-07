package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gitrgoliveira/vault-file-encryption/internal/config"
	"github.com/gitrgoliveira/vault-file-encryption/internal/interfaces"
	"github.com/gitrgoliveira/vault-file-encryption/internal/logger"
	"github.com/gitrgoliveira/vault-file-encryption/internal/model"
)

// Watcher watches directories for file changes
type Watcher struct {
	fsWatcher *fsnotify.Watcher
	queue     interfaces.Queue
	detector  *PartialUploadDetector
	logger    logger.Logger
	mu        sync.RWMutex

	// Configuration
	encryptSourceDir string
	encryptDestDir   string
	decryptSourceDir string
	decryptDestDir   string
}

// Config holds watcher configuration
type Config struct {
	// Encryption directories
	EncryptSourceDir string
	EncryptDestDir   string

	// Decryption directories
	DecryptSourceDir string
	DecryptDestDir   string

	// Stability check duration
	StabilityDuration time.Duration
}

// NewWatcher creates a new file watcher
func NewWatcher(cfg *Config, q interfaces.Queue, log logger.Logger) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fs watcher: %w", err)
	}

	detector := NewPartialUploadDetector(cfg.StabilityDuration)

	w := &Watcher{
		fsWatcher:        fsWatcher,
		queue:            q,
		detector:         detector,
		logger:           log,
		encryptSourceDir: cfg.EncryptSourceDir,
		encryptDestDir:   cfg.EncryptDestDir,
		decryptSourceDir: cfg.DecryptSourceDir,
		decryptDestDir:   cfg.DecryptDestDir,
	}

	return w, nil
}

// UpdateConfig safely updates the watcher's configuration.
func (w *Watcher) UpdateConfig(cfg *config.Config) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	newCfg := &Config{
		EncryptSourceDir: cfg.Encryption.SourceDir,
		EncryptDestDir:   cfg.Encryption.DestDir,
		DecryptSourceDir: cfg.Decryption.SourceDir,
		DecryptDestDir:   cfg.Decryption.DestDir,
	}

	// Update encryption source directory watch
	if newCfg.EncryptSourceDir != w.encryptSourceDir {
		if w.encryptSourceDir != "" {
			if err := w.fsWatcher.Remove(w.encryptSourceDir); err != nil {
				w.logger.Error("Failed to remove old encrypt source dir from watcher", "dir", w.encryptSourceDir, "error", err)
			}
		}
		if newCfg.EncryptSourceDir != "" {
			if err := w.fsWatcher.Add(newCfg.EncryptSourceDir); err != nil {
				return fmt.Errorf("failed to add new encrypt source dir to watcher: %w", err)
			}
			w.logger.Info("Now watching new encryption source directory", "dir", newCfg.EncryptSourceDir)
		}
		w.encryptSourceDir = newCfg.EncryptSourceDir
		w.encryptDestDir = newCfg.EncryptDestDir
	}

	// Update decryption source directory watch
	if newCfg.DecryptSourceDir != w.decryptSourceDir {
		if w.decryptSourceDir != "" {
			if err := w.fsWatcher.Remove(w.decryptSourceDir); err != nil {
				w.logger.Error("Failed to remove old decrypt source dir from watcher", "dir", w.decryptSourceDir, "error", err)
			}
		}
		if newCfg.DecryptSourceDir != "" {
			if err := w.fsWatcher.Add(newCfg.DecryptSourceDir); err != nil {
				return fmt.Errorf("failed to add new decrypt source dir to watcher: %w", err)
			}
			w.logger.Info("Now watching new decryption source directory", "dir", newCfg.DecryptSourceDir)
		}
		w.decryptSourceDir = newCfg.DecryptSourceDir
		w.decryptDestDir = newCfg.DecryptDestDir
	}

	return nil
}

// Start starts watching the configured directories
func (w *Watcher) Start(ctx context.Context) error {
	// Add directories to watch
	w.mu.RLock()
	encryptSrc := w.encryptSourceDir
	decryptSrc := w.decryptSourceDir
	w.mu.RUnlock()

	if encryptSrc != "" {
		if err := w.fsWatcher.Add(encryptSrc); err != nil {
			return fmt.Errorf("failed to watch encrypt source dir: %w", err)
		}
		w.logger.Info("Watching encryption source directory", "dir", encryptSrc)

		// Scan for pre-existing files in encryption source directory
		if err := w.scanDirectory(ctx, encryptSrc, model.OperationEncrypt); err != nil {
			w.logger.Error("Failed to scan encryption source directory", "dir", encryptSrc, "error", err)
		}
	}

	if decryptSrc != "" {
		if err := w.fsWatcher.Add(decryptSrc); err != nil {
			return fmt.Errorf("failed to watch decrypt source dir: %w", err)
		}
		w.logger.Info("Watching decryption source directory", "dir", decryptSrc)

		// Scan for pre-existing files in decryption source directory
		if err := w.scanDirectory(ctx, decryptSrc, model.OperationDecrypt); err != nil {
			w.logger.Error("Failed to scan decryption source directory", "dir", decryptSrc, "error", err)
		}
	}

	// Watch for events
	for {
		select {
		case <-ctx.Done():
			return w.fsWatcher.Close()

		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return nil
			}

			if event.Op&fsnotify.Create == fsnotify.Create {
				w.handleFileCreated(ctx, event.Name)
			}

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return nil
			}
			w.logger.Error("Watcher error", "error", err)
		}
	}
}

// handleFileCreated handles a new file creation event
func (w *Watcher) handleFileCreated(ctx context.Context, filePath string) {
	// Check if it's a file (not directory)
	info, err := os.Stat(filePath)
	if err != nil {
		w.logger.Error("Failed to stat file", "file", filePath, "error", err)
		return
	}

	if info.IsDir() {
		return
	}

	w.mu.RLock()
	defer w.mu.RUnlock()

	// Determine operation type based on directory
	dir := filepath.Dir(filePath)
	var operation model.OperationType
	var destDir string

	//nolint:staticcheck // QF1003: Simple if-else is clearer than tagged switch for two different comparisons
	if dir == w.encryptSourceDir {
		// Skip .enc and .key files in encryption source
		if strings.HasSuffix(filePath, ".enc") || strings.HasSuffix(filePath, ".key") {
			return
		}
		operation = model.OperationEncrypt
		destDir = w.encryptDestDir
	} else if dir == w.decryptSourceDir {
		// Only process .enc files for decryption
		if !strings.HasSuffix(filePath, ".enc") {
			return
		}

		// Check if corresponding .key file exists (based on original filename)
		// example.xlsx.enc -> example.xlsx.key
		keyPath := strings.TrimSuffix(filePath, ".enc") + ".key"

		// Wait for key file to appear (it might be written after .enc file)
		// This handles the race condition where fsnotify detects .enc before .key is written
		keyExists := false
		for i := 0; i < 10; i++ { // Try for up to 1 second
			if _, err := os.Stat(keyPath); err == nil {
				keyExists = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if !keyExists {
			w.logger.Error("Encrypted file without key file", "file", filePath)
			return
		}

		operation = model.OperationDecrypt
		destDir = w.decryptDestDir
	} else {
		return
	}

	w.logger.Info("New file detected", "file", filePath, "operation", operation)

	// Wait for file to be stable (fully uploaded) with context support
	if err := w.detector.WaitForStability(ctx, filePath, 5*time.Minute); err != nil {
		w.logger.Error("File did not stabilize", "file", filePath, "error", err)
		return
	}

	w.logger.Info("File is stable", "file", filePath)

	// Create queue item
	fileName := filepath.Base(filePath)
	destPath := filepath.Join(destDir, fileName)

	item := model.NewItem(operation, filePath, destPath)
	item.FileSize = info.Size()

	// For decryption, set key path
	if operation == model.OperationDecrypt {
		// Key file is based on original filename: example.xlsx.enc -> example.xlsx.key
		item.KeyPath = strings.TrimSuffix(filePath, ".enc") + ".key"
		// Remove .enc from dest path
		item.DestPath = strings.TrimSuffix(destPath, ".enc")
	} else {
		// For encryption, add .enc to destination and set key path based on original filename
		item.DestPath = destPath + ".enc"
		// Key file is based on the original source filename, stored in destination directory
		originalName := filepath.Base(filePath)
		item.KeyPath = filepath.Join(filepath.Dir(destPath), originalName+".key")
	}

	// Enqueue for processing
	if err := w.queue.Enqueue(item); err != nil {
		w.logger.Error("Failed to enqueue item", "file", filePath, "error", err)
		return
	}

	w.logger.Info("File queued for processing", "file", filePath, "id", item.ID)
}

// Stop stops the watcher
func (w *Watcher) Stop() error {
	return w.fsWatcher.Close()
}

// scanDirectory scans a directory for pre-existing files and queues them for processing
func (w *Watcher) scanDirectory(ctx context.Context, dir string, operation model.OperationType) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	w.mu.RLock()
	defer w.mu.RUnlock()

	var destDir string
	if operation == model.OperationEncrypt {
		destDir = w.encryptDestDir
	} else {
		destDir = w.decryptDestDir
	}

	filesQueued := 0
	for _, entry := range entries {
		// Skip directories and subdirectories (archive, failed, dlq, etc.)
		if entry.IsDir() {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())

		// Apply same filtering as handleFileCreated
		if operation == model.OperationEncrypt {
			// Skip .enc and .key files in encryption source
			if strings.HasSuffix(filePath, ".enc") || strings.HasSuffix(filePath, ".key") || strings.HasSuffix(filePath, ".sha256") {
				continue
			}
		} else if operation == model.OperationDecrypt {
			// Only process .enc files for decryption
			if !strings.HasSuffix(filePath, ".enc") {
				continue
			}

			// Check if corresponding .key file exists
			keyPath := strings.TrimSuffix(filePath, ".enc") + ".key"
			if _, err := os.Stat(keyPath); os.IsNotExist(err) {
				w.logger.Error("Encrypted file without key file", "file", filePath)
				continue
			}
		}

		// Get file info
		info, err := entry.Info()
		if err != nil {
			w.logger.Error("Failed to get file info", "file", filePath, "error", err)
			continue
		}

		// Wait for file to be stable (fully uploaded) with context support
		if err := w.detector.WaitForStability(ctx, filePath, 5*time.Minute); err != nil {
			w.logger.Error("File did not stabilize", "file", filePath, "error", err)
			continue
		}

		w.logger.Info("Pre-existing file found and stable", "file", filePath, "operation", operation)

		// Create queue item
		fileName := filepath.Base(filePath)
		destPath := filepath.Join(destDir, fileName)

		item := model.NewItem(operation, filePath, destPath)
		item.FileSize = info.Size()

		// For decryption, set key path
		if operation == model.OperationDecrypt {
			item.KeyPath = strings.TrimSuffix(filePath, ".enc") + ".key"
			item.DestPath = strings.TrimSuffix(destPath, ".enc")
		} else {
			// For encryption, add .enc to destination and set key path based on original filename
			item.DestPath = destPath + ".enc"
			originalName := filepath.Base(filePath)
			item.KeyPath = filepath.Join(filepath.Dir(destPath), originalName+".key")
		}

		// Enqueue for processing
		if err := w.queue.Enqueue(item); err != nil {
			w.logger.Error("Failed to enqueue item", "file", filePath, "error", err)
			continue
		}

		w.logger.Info("Pre-existing file queued for processing", "file", filePath, "id", item.ID)
		filesQueued++
	}

	if filesQueued > 0 {
		w.logger.Info("Pre-existing files queued", "count", filesQueued, "operation", operation)
	}

	return nil
}
