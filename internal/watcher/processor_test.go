package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gitrgoliveira/vault-file-encryption/internal/config"
	"github.com/gitrgoliveira/vault-file-encryption/internal/model"

	"github.com/gitrgoliveira/vault-file-encryption/internal/crypto"
	"github.com/gitrgoliveira/vault-file-encryption/internal/logger"
	"github.com/gitrgoliveira/vault-file-encryption/internal/queue"
	"github.com/gitrgoliveira/vault-file-encryption/internal/vault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock Vault client for testing
type mockVaultClient struct{}

func (m *mockVaultClient) GenerateDataKey() (*vault.DataKey, error) {
	// Return a mock plaintext DEK (base64 encoded 32 bytes)
	return &vault.DataKey{
		Plaintext:  "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=",
		Ciphertext: "vault:v1:mock-encrypted-dek",
		KeyVersion: 1,
	}, nil
}

func (m *mockVaultClient) DecryptDataKey(ciphertext string) (*vault.DataKey, error) {
	// Return the same mock plaintext DEK
	return &vault.DataKey{
		Plaintext:  "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=",
		Ciphertext: ciphertext,
		KeyVersion: 1,
	}, nil
}

func setupTestProcessor(t *testing.T, cfg *ProcessorConfig) (*Processor, *queue.Queue, string) {
	tmpDir := t.TempDir()

	// Set up directories
	if cfg.ArchiveDir == "" {
		cfg.ArchiveDir = filepath.Join(tmpDir, "archive")
	}
	if cfg.FailedDir == "" {
		cfg.FailedDir = filepath.Join(tmpDir, "failed")
	}
	if cfg.DLQDir == "" {
		cfg.DLQDir = filepath.Join(tmpDir, "dlq")
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

	// Create crypto components with mock vault client
	vaultClient := &mockVaultClient{}
	encryptor := crypto.NewEncryptor(vaultClient, nil)
	decryptor := crypto.NewDecryptor(vaultClient, nil)

	// Create logger
	log, err := logger.New("error", "/dev/null") // Log to null device in tests
	require.NoError(t, err)

	// Create processor
	processor, err := NewProcessor(cfg, q, encryptor, decryptor, log)
	require.NoError(t, err)

	return processor, q, tmpDir
}

func TestNewProcessor(t *testing.T) {
	cfg := &ProcessorConfig{
		SourceFileBehavior: "archive",
		CalculateChecksum:  true,
		VerifyChecksum:     true,
	}

	processor, _, tmpDir := setupTestProcessor(t, cfg)
	require.NotNil(t, processor)

	// Verify directories were created
	assert.DirExists(t, filepath.Join(tmpDir, "archive"))
	assert.DirExists(t, filepath.Join(tmpDir, "failed"))
	assert.DirExists(t, filepath.Join(tmpDir, "dlq"))
}

func TestProcessor_UpdateConfig(t *testing.T) {
	cfg := &ProcessorConfig{
		SourceFileBehavior: "archive",
		CalculateChecksum:  false,
	}

	processor, _, tmpDir := setupTestProcessor(t, cfg)

	// Create a full config.Config for UpdateConfig
	newAppCfg := &config.Config{
		Encryption: config.EncryptionConfig{
			SourceDir:          tmpDir,
			SourceFileBehavior: "delete",
			CalculateChecksum:  true,
		},
		Decryption: &config.DecryptionConfig{
			VerifyChecksum: true,
		},
	}

	processor.UpdateConfig(newAppCfg)

	// The UpdateConfig should not error and should update internal state
	// We can't easily verify internal state changes without exposing more internals
	// so we just ensure the method completes successfully
}

func TestProcessor_EncryptFile(t *testing.T) {
	cfg := &ProcessorConfig{
		SourceFileBehavior: "archive",
		CalculateChecksum:  true,
	}

	processor, q, tmpDir := setupTestProcessor(t, cfg)

	// Create test file
	sourceFile := filepath.Join(tmpDir, "source.txt")
	testData := []byte("This is test data for encryption")
	err := os.WriteFile(sourceFile, testData, 0600)
	require.NoError(t, err)

	// Create queue item
	destFile := filepath.Join(tmpDir, "encrypted.enc")
	item := model.NewItem(model.OperationEncrypt, sourceFile, destFile)
	item.KeyPath = destFile + ".key"
	info, _ := os.Stat(sourceFile)
	item.FileSize = info.Size()

	// Enqueue and process
	err = q.Enqueue(item)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Process the item
	processedItem := q.Dequeue()
	require.NotNil(t, processedItem)

	processor.processItem(ctx, processedItem)

	// Verify encrypted file and key were created
	assert.FileExists(t, destFile)
	assert.FileExists(t, destFile+".key")

	// Verify checksum was created
	assert.FileExists(t, destFile+".sha256")

	// Verify source was archived
	archiveFile := filepath.Join(cfg.ArchiveDir, filepath.Base(sourceFile))
	assert.FileExists(t, archiveFile)
	assert.NoFileExists(t, sourceFile)
}

func TestProcessor_EncryptFile_Delete(t *testing.T) {
	cfg := &ProcessorConfig{
		SourceFileBehavior: "delete",
		CalculateChecksum:  false,
	}

	processor, q, tmpDir := setupTestProcessor(t, cfg)

	// Create test file
	sourceFile := filepath.Join(tmpDir, "source.txt")
	testData := []byte("Test data")
	err := os.WriteFile(sourceFile, testData, 0600)
	require.NoError(t, err)

	// Create queue item
	destFile := filepath.Join(tmpDir, "encrypted.enc")
	item := model.NewItem(model.OperationEncrypt, sourceFile, destFile)
	item.KeyPath = destFile + ".key"
	info, _ := os.Stat(sourceFile)
	item.FileSize = info.Size()

	// Enqueue and process
	err = q.Enqueue(item)
	require.NoError(t, err)

	ctx := context.Background()
	processedItem := q.Dequeue()
	require.NotNil(t, processedItem)

	processor.processItem(ctx, processedItem)

	// Verify source was deleted (not archived)
	assert.NoFileExists(t, sourceFile)

	// Verify no checksum was created
	assert.NoFileExists(t, destFile+".sha256")
}

func TestProcessor_DecryptFile(t *testing.T) {
	cfg := &ProcessorConfig{
		SourceFileBehavior: "delete",
		CalculateChecksum:  true,
		VerifyChecksum:     true,
	}

	processor, q, tmpDir := setupTestProcessor(t, cfg)

	// First encrypt a file
	sourceFile := filepath.Join(tmpDir, "original.txt")
	testData := []byte("This is test data for decryption")
	err := os.WriteFile(sourceFile, testData, 0600)
	require.NoError(t, err)

	encryptedFile := filepath.Join(tmpDir, "encrypted.enc")
	encryptItem := model.NewItem(model.OperationEncrypt, sourceFile, encryptedFile)
	encryptItem.KeyPath = encryptedFile + ".key"
	info, _ := os.Stat(sourceFile)
	encryptItem.FileSize = info.Size()

	err = q.Enqueue(encryptItem)
	require.NoError(t, err)

	ctx := context.Background()
	processedEncryptItem := q.Dequeue()
	require.NotNil(t, processedEncryptItem)

	processor.processItem(ctx, processedEncryptItem)

	// Now decrypt the file
	decryptedFile := filepath.Join(tmpDir, "decrypted.txt")
	decryptItem := model.NewItem(model.OperationDecrypt, encryptedFile, decryptedFile)
	decryptItem.KeyPath = encryptedFile + ".key"
	encInfo, _ := os.Stat(encryptedFile)
	decryptItem.FileSize = encInfo.Size()

	err = q.Enqueue(decryptItem)
	require.NoError(t, err)

	// Re-create the encrypted file (since it was deleted)
	err = os.WriteFile(sourceFile, testData, 0600)
	require.NoError(t, err)

	processedEncryptItem2 := q.Dequeue()
	require.NotNil(t, processedEncryptItem2)

	processor.processItem(ctx, processedEncryptItem2)

	err = q.Enqueue(decryptItem)
	require.NoError(t, err)

	processedDecryptItem := q.Dequeue()
	require.NotNil(t, processedDecryptItem)

	processor.processItem(ctx, processedDecryptItem)

	// Verify decrypted file exists
	assert.FileExists(t, decryptedFile)

	// Verify content matches
	decryptedData, err := os.ReadFile(decryptedFile)
	require.NoError(t, err)
	assert.Equal(t, testData, decryptedData)
}

func TestProcessor_Start_ContextCancellation(t *testing.T) {
	cfg := &ProcessorConfig{
		SourceFileBehavior: "archive",
	}

	processor, _, _ := setupTestProcessor(t, cfg)

	// Start processor with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := processor.Start(ctx)
	assert.NoError(t, err)
}

func TestProcessor_HandleSourceFile_UnknownBehavior(t *testing.T) {
	cfg := &ProcessorConfig{
		SourceFileBehavior: "unknown-behavior",
	}

	processor, _, tmpDir := setupTestProcessor(t, cfg)

	// Create test file
	sourceFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(sourceFile, []byte("test"), 0600)
	require.NoError(t, err)

	// Call handleSourceFile - it should log error but not crash
	processor.FileHandler.HandleSourceFile(sourceFile)

	// File should still exist (unknown behavior means no action)
	assert.FileExists(t, sourceFile)
}

func TestProcessor_MoveToFailed(t *testing.T) {
	cfg := &ProcessorConfig{
		SourceFileBehavior: "archive",
	}

	processor, _, tmpDir := setupTestProcessor(t, cfg)

	// Create test file
	sourceFile := filepath.Join(tmpDir, "failed.txt")
	err := os.WriteFile(sourceFile, []byte("test"), 0600)
	require.NoError(t, err)

	// Move to failed directory
	processor.FileHandler.MoveToFailed(sourceFile)

	// Verify file was moved
	failedFile := filepath.Join(cfg.FailedDir, filepath.Base(sourceFile))
	assert.FileExists(t, failedFile)
	assert.NoFileExists(t, sourceFile)
}

func TestProcessor_MoveToDLQ(t *testing.T) {
	cfg := &ProcessorConfig{
		SourceFileBehavior: "archive",
	}

	processor, _, tmpDir := setupTestProcessor(t, cfg)

	// Create test file
	sourceFile := filepath.Join(tmpDir, "dlq.txt")
	err := os.WriteFile(sourceFile, []byte("test"), 0600)
	require.NoError(t, err)

	item := model.NewItem(model.OperationEncrypt, sourceFile, "")

	// Move to DLQ
	processor.FileHandler.MoveToDLQ(item)

	// Verify file was moved
	dlqFile := filepath.Join(cfg.DLQDir, filepath.Base(sourceFile))
	assert.FileExists(t, dlqFile)
	assert.NoFileExists(t, sourceFile)
}

func TestProcessor_MoveToFailed_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &ProcessorConfig{
		SourceFileBehavior: "archive",
		ArchiveDir:         filepath.Join(tmpDir, "archive"),
		FailedDir:          "", // Empty - should not move files
		DLQDir:             filepath.Join(tmpDir, "dlq"),
	}

	processor, _, _ := setupTestProcessorWithExactConfig(t, cfg)

	// Create test file
	sourceFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(sourceFile, []byte("test"), 0600)
	require.NoError(t, err)

	// Should not crash when failedDir is empty
	processor.FileHandler.MoveToFailed(sourceFile)

	// File should still exist
	assert.FileExists(t, sourceFile)
}

func setupTestProcessorWithExactConfig(t *testing.T, cfg *ProcessorConfig) (*Processor, *queue.Queue, string) {
	tmpDir := t.TempDir()

	// Create queue
	queueCfg := &queue.Config{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   5 * time.Second,
		StatePath:  filepath.Join(tmpDir, "queue.json"),
	}
	q, err := queue.NewQueue(queueCfg)
	require.NoError(t, err)

	// Create crypto components with mock vault client
	vaultClient := &mockVaultClient{}
	encryptor := crypto.NewEncryptor(vaultClient, nil)
	decryptor := crypto.NewDecryptor(vaultClient, nil)

	// Create logger
	log, err := logger.New("error", "/dev/null") // Log to null device in tests
	require.NoError(t, err)

	// Create processor with exact config (don't override empty values)
	processor, err := NewProcessor(cfg, q, encryptor, decryptor, log)
	require.NoError(t, err)

	return processor, q, tmpDir
}

func TestProcessor_MoveToDLQ_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &ProcessorConfig{
		SourceFileBehavior: "archive",
		ArchiveDir:         filepath.Join(tmpDir, "archive"),
		FailedDir:          filepath.Join(tmpDir, "failed"),
		DLQDir:             "", // Empty - should not move files
	}

	processor, _, _ := setupTestProcessorWithExactConfig(t, cfg)

	// Create test file
	sourceFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(sourceFile, []byte("test"), 0600)
	require.NoError(t, err)

	item := model.NewItem(model.OperationEncrypt, sourceFile, "")

	// Should not crash when dlqDir is empty
	processor.FileHandler.MoveToDLQ(item)

	// File should still exist
	assert.FileExists(t, sourceFile)
}

func TestProcessor_ProcessItem_EncryptionFailure(t *testing.T) {
	cfg := &ProcessorConfig{
		SourceFileBehavior: "archive",
		CalculateChecksum:  false,
	}

	processor, q, tmpDir := setupTestProcessor(t, cfg)

	// Create a non-existent source file path (will cause encryption to fail)
	sourceFile := filepath.Join(tmpDir, "nonexistent.txt")
	destFile := filepath.Join(tmpDir, "encrypted.enc")

	item := model.NewItem(model.OperationEncrypt, sourceFile, destFile)
	item.KeyPath = destFile + ".key"
	item.FileSize = 0

	err := q.Enqueue(item)
	require.NoError(t, err)

	ctx := context.Background()
	processedItem := q.Dequeue()
	require.NotNil(t, processedItem)

	// Process item - should fail
	processor.processItem(ctx, processedItem)

	// Item should have been marked as failed and requeued
	assert.Greater(t, processedItem.AttemptCount, 0)
}

func TestProcessor_DecryptFile_ChecksumVerificationFailure(t *testing.T) {
	cfg := &ProcessorConfig{
		SourceFileBehavior: "delete",
		CalculateChecksum:  true,
		VerifyChecksum:     true,
	}

	processor, q, tmpDir := setupTestProcessor(t, cfg)

	// First encrypt a file
	sourceFile := filepath.Join(tmpDir, "original.txt")
	testData := []byte("This is test data")
	err := os.WriteFile(sourceFile, testData, 0600)
	require.NoError(t, err)

	encryptedFile := filepath.Join(tmpDir, "encrypted.enc")
	encryptItem := model.NewItem(model.OperationEncrypt, sourceFile, encryptedFile)
	encryptItem.KeyPath = encryptedFile + ".key"
	info, _ := os.Stat(sourceFile)
	encryptItem.FileSize = info.Size()

	err = q.Enqueue(encryptItem)
	require.NoError(t, err)

	ctx := context.Background()
	processedEncryptItem := q.Dequeue()
	require.NotNil(t, processedEncryptItem)

	processor.processItem(ctx, processedEncryptItem)

	// Tamper with the checksum file
	checksumPath := encryptedFile + ".sha256"
	err = os.WriteFile(checksumPath, []byte("invalid-checksum-value"), 0600)
	require.NoError(t, err)

	// Re-create source for another encryption
	err = os.WriteFile(sourceFile, testData, 0600)
	require.NoError(t, err)

	processedEncryptItem2 := q.Dequeue()
	if processedEncryptItem2 != nil {
		processor.processItem(ctx, processedEncryptItem2)
	}

	// Now decrypt with invalid checksum - should fail verification
	decryptedFile := filepath.Join(tmpDir, "decrypted.txt")
	decryptItem := model.NewItem(model.OperationDecrypt, encryptedFile, decryptedFile)
	decryptItem.KeyPath = encryptedFile + ".key"

	err = q.Enqueue(decryptItem)
	require.NoError(t, err)

	processedDecryptItem := q.Dequeue()
	require.NotNil(t, processedDecryptItem)

	// Process - should fail due to invalid checksum
	processor.processItem(ctx, processedDecryptItem)

	// Verify item was requeued due to checksum failure
	assert.Greater(t, processedDecryptItem.AttemptCount, 0)
}

func TestProcessor_ProcessItem_UnknownOperation(t *testing.T) {
	cfg := &ProcessorConfig{
		SourceFileBehavior: "archive",
	}

	processor, q, tmpDir := setupTestProcessor(t, cfg)

	// Create test file
	sourceFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(sourceFile, []byte("test"), 0600)
	require.NoError(t, err)

	// Create item with unknown operation
	item := model.NewItem("unknown-operation", sourceFile, "")
	item.FileSize = 4

	err = q.Enqueue(item)
	require.NoError(t, err)

	ctx := context.Background()
	processedItem := q.Dequeue()
	require.NotNil(t, processedItem)

	// Process - should fail with unknown operation
	processor.processItem(ctx, processedItem)

	// Item should have been requeued due to failure
	assert.Greater(t, processedItem.AttemptCount, 0)
}
