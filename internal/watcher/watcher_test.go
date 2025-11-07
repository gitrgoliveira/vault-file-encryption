package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gitrgoliveira/vault-file-encryption/internal/config"
	"github.com/gitrgoliveira/vault-file-encryption/internal/model"

	"github.com/gitrgoliveira/vault-file-encryption/internal/logger"
	"github.com/gitrgoliveira/vault-file-encryption/internal/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestWatcher(t *testing.T, cfg *Config) (*Watcher, *queue.Queue, string) {
	tmpDir := t.TempDir()

	// Create directories
	encryptSrc := filepath.Join(tmpDir, "encrypt-src")
	encryptDest := filepath.Join(tmpDir, "encrypt-dest")
	decryptSrc := filepath.Join(tmpDir, "decrypt-src")
	decryptDest := filepath.Join(tmpDir, "decrypt-dest")

	require.NoError(t, os.MkdirAll(encryptSrc, 0750))
	require.NoError(t, os.MkdirAll(encryptDest, 0750))
	require.NoError(t, os.MkdirAll(decryptSrc, 0750))
	require.NoError(t, os.MkdirAll(decryptDest, 0750))

	if cfg == nil {
		cfg = &Config{
			EncryptSourceDir:  encryptSrc,
			EncryptDestDir:    encryptDest,
			DecryptSourceDir:  decryptSrc,
			DecryptDestDir:    decryptDest,
			StabilityDuration: 100 * time.Millisecond,
		}
	} else {
		if cfg.EncryptSourceDir == "" {
			cfg.EncryptSourceDir = encryptSrc
		}
		if cfg.EncryptDestDir == "" {
			cfg.EncryptDestDir = encryptDest
		}
		if cfg.DecryptSourceDir == "" {
			cfg.DecryptSourceDir = decryptSrc
		}
		if cfg.DecryptDestDir == "" {
			cfg.DecryptDestDir = decryptDest
		}
		if cfg.StabilityDuration == 0 {
			cfg.StabilityDuration = 100 * time.Millisecond
		}
	}

	// Create queue
	queueCfg := &queue.Config{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   5 * time.Second,
		StatePath:  filepath.Join(tmpDir, "queue.json"),
	}
	q, err := queue.NewQueue(queueCfg)
	require.NoError(t, err)

	// Create logger
	log, err := logger.New("error", "/dev/null")
	require.NoError(t, err)

	// Create watcher
	watcher, err := NewWatcher(cfg, q, log)
	require.NoError(t, err)

	return watcher, q, tmpDir
}

func TestNewWatcher(t *testing.T) {
	watcher, q, _ := setupTestWatcher(t, nil)
	require.NotNil(t, watcher)
	require.NotNil(t, watcher.fsWatcher)
	require.NotNil(t, watcher.detector)
	require.NotNil(t, watcher.queue)
	assert.Equal(t, q, watcher.queue)
}

func TestWatcher_Start_Stop(t *testing.T) {
	watcher, _, _ := setupTestWatcher(t, nil)

	// Start watcher with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := watcher.Start(ctx)
	assert.NoError(t, err)
}

func TestWatcher_HandleFileCreated_EncryptOperation(t *testing.T) {
	watcher, q, tmpDir := setupTestWatcher(t, nil)

	// Create a file in the encryption source directory
	encryptSrc := filepath.Join(tmpDir, "encrypt-src")
	testFile := filepath.Join(encryptSrc, "test.txt")
	err := os.WriteFile(testFile, []byte("test data"), 0600)
	require.NoError(t, err)

	// Handle the file creation
	ctx := context.Background()
	watcher.handleFileCreated(ctx, testFile)

	// Wait a bit for processing
	time.Sleep(200 * time.Millisecond)

	// Check if item was queued
	item := q.Dequeue()
	require.NotNil(t, item)
	assert.Equal(t, model.OperationEncrypt, item.Operation)
	assert.Equal(t, testFile, item.SourcePath)
	assert.Contains(t, item.DestPath, "encrypt-dest")
	assert.Contains(t, item.KeyPath, ".key")
}

func TestWatcher_HandleFileCreated_SkipEncFiles(t *testing.T) {
	watcher, q, tmpDir := setupTestWatcher(t, nil)

	// Create .enc file in encryption source (should be skipped)
	encryptSrc := filepath.Join(tmpDir, "encrypt-src")
	encFile := filepath.Join(encryptSrc, "test.enc")
	err := os.WriteFile(encFile, []byte("encrypted"), 0600)
	require.NoError(t, err)

	ctx := context.Background()
	watcher.handleFileCreated(ctx, encFile)

	time.Sleep(200 * time.Millisecond)

	// Should not be queued
	item := q.Dequeue()
	assert.Nil(t, item)
}

func TestWatcher_HandleFileCreated_SkipKeyFiles(t *testing.T) {
	watcher, q, tmpDir := setupTestWatcher(t, nil)

	// Create .key file in encryption source (should be skipped)
	encryptSrc := filepath.Join(tmpDir, "encrypt-src")
	keyFile := filepath.Join(encryptSrc, "test.key")
	err := os.WriteFile(keyFile, []byte("key"), 0600)
	require.NoError(t, err)

	ctx := context.Background()
	watcher.handleFileCreated(ctx, keyFile)

	time.Sleep(200 * time.Millisecond)

	// Should not be queued
	item := q.Dequeue()
	assert.Nil(t, item)
}

func TestWatcher_HandleFileCreated_DecryptOperation(t *testing.T) {
	watcher, q, tmpDir := setupTestWatcher(t, nil)

	// Create .enc and .key files in decryption source
	decryptSrc := filepath.Join(tmpDir, "decrypt-src")
	encFile := filepath.Join(decryptSrc, "test.enc")
	keyFile := filepath.Join(decryptSrc, "test.key")

	err := os.WriteFile(encFile, []byte("encrypted data"), 0600)
	require.NoError(t, err)
	err = os.WriteFile(keyFile, []byte("vault:v1:key"), 0600)
	require.NoError(t, err)

	// Handle the file creation
	ctx := context.Background()
	watcher.handleFileCreated(ctx, encFile)

	// Wait for stability check
	time.Sleep(200 * time.Millisecond)

	// Check if item was queued
	item := q.Dequeue()
	require.NotNil(t, item)
	assert.Equal(t, model.OperationDecrypt, item.Operation)
	assert.Equal(t, encFile, item.SourcePath)
	assert.Contains(t, item.DestPath, "decrypt-dest")
	assert.Equal(t, keyFile, item.KeyPath)
	// Dest path should not have .enc extension
	assert.NotContains(t, item.DestPath, ".enc")
}

func TestWatcher_HandleFileCreated_DecryptWithoutKey(t *testing.T) {
	watcher, q, tmpDir := setupTestWatcher(t, nil)

	// Create .enc file WITHOUT .key file in decryption source
	decryptSrc := filepath.Join(tmpDir, "decrypt-src")
	encFile := filepath.Join(decryptSrc, "test.enc")

	err := os.WriteFile(encFile, []byte("encrypted data"), 0600)
	require.NoError(t, err)

	ctx := context.Background()
	watcher.handleFileCreated(ctx, encFile)

	time.Sleep(200 * time.Millisecond)

	// Should not be queued (missing key file)
	item := q.Dequeue()
	assert.Nil(t, item)
}

func TestWatcher_HandleFileCreated_SkipNonEncFilesInDecryptDir(t *testing.T) {
	watcher, q, tmpDir := setupTestWatcher(t, nil)

	// Create non-.enc file in decryption source (should be skipped)
	decryptSrc := filepath.Join(tmpDir, "decrypt-src")
	txtFile := filepath.Join(decryptSrc, "test.txt")
	err := os.WriteFile(txtFile, []byte("plain text"), 0600)
	require.NoError(t, err)

	ctx := context.Background()
	watcher.handleFileCreated(ctx, txtFile)

	time.Sleep(200 * time.Millisecond)

	// Should not be queued
	item := q.Dequeue()
	assert.Nil(t, item)
}

func TestWatcher_HandleFileCreated_Directory(t *testing.T) {
	watcher, q, tmpDir := setupTestWatcher(t, nil)

	// Create a subdirectory in encryption source
	encryptSrc := filepath.Join(tmpDir, "encrypt-src")
	subDir := filepath.Join(encryptSrc, "subdir")
	err := os.MkdirAll(subDir, 0750)
	require.NoError(t, err)

	ctx := context.Background()
	watcher.handleFileCreated(ctx, subDir)

	time.Sleep(200 * time.Millisecond)

	// Directories should not be queued
	item := q.Dequeue()
	assert.Nil(t, item)
}

func TestWatcher_HandleFileCreated_NonExistentFile(t *testing.T) {
	watcher, q, tmpDir := setupTestWatcher(t, nil)

	// Try to handle a non-existent file
	nonExistent := filepath.Join(tmpDir, "encrypt-src", "nonexistent.txt")

	ctx := context.Background()
	watcher.handleFileCreated(ctx, nonExistent)

	time.Sleep(200 * time.Millisecond)

	// Should not crash, and nothing should be queued
	item := q.Dequeue()
	assert.Nil(t, item)
}

func TestWatcher_HandleFileCreated_ContextCancellation(t *testing.T) {
	watcher, q, tmpDir := setupTestWatcher(t, nil)

	// Create a file
	encryptSrc := filepath.Join(tmpDir, "encrypt-src")
	testFile := filepath.Join(encryptSrc, "test.txt")
	err := os.WriteFile(testFile, []byte("test data"), 0600)
	require.NoError(t, err)

	// Use cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	watcher.handleFileCreated(ctx, testFile)

	time.Sleep(200 * time.Millisecond)

	// Should not be queued due to context cancellation during stability check
	item := q.Dequeue()
	assert.Nil(t, item)
}

func TestWatcher_UpdateConfig(t *testing.T) {
	watcher, _, tmpDir := setupTestWatcher(t, nil)

	// Create new directories
	newEncryptSrc := filepath.Join(tmpDir, "new-encrypt-src")
	newEncryptDest := filepath.Join(tmpDir, "new-encrypt-dest")
	newDecryptSrc := filepath.Join(tmpDir, "new-decrypt-src")
	newDecryptDest := filepath.Join(tmpDir, "new-decrypt-dest")

	require.NoError(t, os.MkdirAll(newEncryptSrc, 0750))
	require.NoError(t, os.MkdirAll(newEncryptDest, 0750))
	require.NoError(t, os.MkdirAll(newDecryptSrc, 0750))
	require.NoError(t, os.MkdirAll(newDecryptDest, 0750))

	// Start watcher first
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startDone := make(chan struct{})
	go func() {
		close(startDone) // Signal that Start() has been called
		_ = watcher.Start(ctx)
	}()

	// Wait for Start() to begin before updating config
	<-startDone
	time.Sleep(200 * time.Millisecond)

	// Update config with a full config.Config
	newAppCfg := &config.Config{
		Encryption: config.EncryptionConfig{
			SourceDir: newEncryptSrc,
			DestDir:   newEncryptDest,
		},
		Decryption: &config.DecryptionConfig{
			SourceDir: newDecryptSrc,
			DestDir:   newDecryptDest,
		},
		Queue: config.QueueConfig{
			StabilityDuration: config.Duration(200 * time.Millisecond),
		},
	}

	err := watcher.UpdateConfig(newAppCfg)
	assert.NoError(t, err)

	// Verify config was updated
	watcher.mu.RLock()
	assert.Equal(t, newEncryptSrc, watcher.encryptSourceDir)
	assert.Equal(t, newEncryptDest, watcher.encryptDestDir)
	assert.Equal(t, newDecryptSrc, watcher.decryptSourceDir)
	assert.Equal(t, newDecryptDest, watcher.decryptDestDir)
	watcher.mu.RUnlock()
}

func TestWatcher_UpdateConfig_SameDirectory(t *testing.T) {
	watcher, _, tmpDir := setupTestWatcher(t, nil)

	// Start watcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = watcher.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Update with same directories (should not error)
	sameCfg := &config.Config{
		Encryption: config.EncryptionConfig{
			SourceDir: filepath.Join(tmpDir, "encrypt-src"),
			DestDir:   filepath.Join(tmpDir, "encrypt-dest"),
		},
		Decryption: &config.DecryptionConfig{
			SourceDir: filepath.Join(tmpDir, "decrypt-src"),
			DestDir:   filepath.Join(tmpDir, "decrypt-dest"),
		},
		Queue: config.QueueConfig{
			StabilityDuration: config.Duration(100 * time.Millisecond),
		},
	}

	err := watcher.UpdateConfig(sameCfg)
	assert.NoError(t, err)
}

func TestWatcher_UpdateConfig_InvalidDirectory(t *testing.T) {
	watcher, _, _ := setupTestWatcher(t, nil)

	// Start watcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = watcher.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Try to update with non-existent directory
	invalidCfg := &config.Config{
		Encryption: config.EncryptionConfig{
			SourceDir: "/nonexistent/path/that/does/not/exist",
			DestDir:   "/another/invalid/path",
		},
		Decryption: &config.DecryptionConfig{
			SourceDir: filepath.Join(os.TempDir(), "decrypt-src"),
			DestDir:   filepath.Join(os.TempDir(), "decrypt-dest"),
		},
		Queue: config.QueueConfig{
			StabilityDuration: config.Duration(100 * time.Millisecond),
		},
	}

	err := watcher.UpdateConfig(invalidCfg)
	// Should error when trying to watch non-existent directory
	assert.Error(t, err)
}

func TestWatcher_Stop(t *testing.T) {
	watcher, _, _ := setupTestWatcher(t, nil)

	// Start watcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = watcher.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Stop watcher
	err := watcher.Stop()
	assert.NoError(t, err)
}

func TestWatcher_FileFromOtherDirectory(t *testing.T) {
	watcher, q, tmpDir := setupTestWatcher(t, nil)

	// Create file in a different directory (not watched)
	otherDir := filepath.Join(tmpDir, "other")
	require.NoError(t, os.MkdirAll(otherDir, 0750))

	testFile := filepath.Join(otherDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test"), 0600)
	require.NoError(t, err)

	ctx := context.Background()
	watcher.handleFileCreated(ctx, testFile)

	time.Sleep(200 * time.Millisecond)

	// Should not be queued (not from watched directories)
	item := q.Dequeue()
	assert.Nil(t, item)
}
