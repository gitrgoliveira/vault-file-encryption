//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gitrgoliveira/vault-file-encryption/internal/config"
	"github.com/gitrgoliveira/vault-file-encryption/internal/crypto"
	"github.com/gitrgoliveira/vault-file-encryption/internal/vault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEndToEndEncryption tests the complete encryption workflow
func TestEndToEndEncryption(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check for required environment variables
	vaultAddr := os.Getenv("VAULT_ADDR")
	vaultToken := os.Getenv("VAULT_TOKEN")

	if vaultAddr == "" || vaultToken == "" {
		t.Skip("Skipping integration test: VAULT_ADDR or VAULT_TOKEN not set")
	}

	// Set Vault token for API client
	os.Setenv("VAULT_TOKEN", vaultToken)

	// Create temporary directories
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	destDir := filepath.Join(tmpDir, "dest")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(destDir, 0755))

	// Create a test file
	testContent := []byte("This is a test file for end-to-end encryption testing.\nIt has multiple lines.\nAnd some data to encrypt.\n")
	testFile := filepath.Join(sourceDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, testContent, 0644))

	// Initialize Vault client
	vaultClient, err := vault.NewClient(&vault.Config{
		AgentAddress: vaultAddr,
		TransitMount: "transit",
		KeyName:      "file-encryption-key",
		Timeout:      30 * time.Second,
	})
	require.NoError(t, err)

	// Create encryptor
	encryptor := crypto.NewEncryptor(vaultClient, nil)

	// Encrypt the file
	encryptedFile := filepath.Join(destDir, "test.txt.enc")
	keyFile := filepath.Join(destDir, "test.txt.key")

	ctx := context.Background()
	encryptedKey, err := encryptor.EncryptFile(ctx, testFile, encryptedFile, nil)
	require.NoError(t, err)
	require.NotEmpty(t, encryptedKey)

	// Save the encrypted key to file
	require.NoError(t, os.WriteFile(keyFile, []byte(encryptedKey), 0644))

	// Verify encrypted file exists and is different from original
	encryptedData, err := os.ReadFile(encryptedFile)
	require.NoError(t, err)
	assert.NotEqual(t, testContent, encryptedData)

	// Create decryptor
	decryptor := crypto.NewDecryptor(vaultClient, nil)

	// Decrypt the file
	decryptedFile := filepath.Join(tmpDir, "decrypted.txt")
	err = decryptor.DecryptFile(ctx, encryptedFile, keyFile, decryptedFile, nil)
	require.NoError(t, err)

	// Verify decrypted content matches original
	decryptedContent, err := os.ReadFile(decryptedFile)
	require.NoError(t, err)
	assert.Equal(t, testContent, decryptedContent)
}

// TestEndToEndWithChecksum tests encryption/decryption with checksum validation
func TestEndToEndWithChecksum(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check for required environment variables
	vaultAddr := os.Getenv("VAULT_ADDR")
	vaultToken := os.Getenv("VAULT_TOKEN")

	if vaultAddr == "" || vaultToken == "" {
		t.Skip("Skipping integration test: VAULT_ADDR or VAULT_TOKEN not set")
	}

	// Set Vault token for API client
	os.Setenv("VAULT_TOKEN", vaultToken)

	// Create temporary directories
	tmpDir := t.TempDir()
	testContent := []byte("Test content for checksum validation")
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, testContent, 0644))

	// Calculate original checksum
	originalChecksum, err := crypto.CalculateChecksum(testFile)
	require.NoError(t, err)

	// Initialize Vault client
	vaultClient, err := vault.NewClient(&vault.Config{
		AgentAddress: vaultAddr,
		TransitMount: "transit",
		KeyName:      "file-encryption-key",
		Timeout:      30 * time.Second,
	})
	require.NoError(t, err)

	// Create encryptor
	encryptor := crypto.NewEncryptor(vaultClient, nil)

	// Encrypt the file
	encryptedFile := filepath.Join(tmpDir, "test.txt.enc")
	keyFile := filepath.Join(tmpDir, "test.txt.key")
	ctx := context.Background()
	encryptedKey, err := encryptor.EncryptFile(ctx, testFile, encryptedFile, nil)
	require.NoError(t, err)

	// Save the encrypted key to file
	require.NoError(t, os.WriteFile(keyFile, []byte(encryptedKey), 0644))

	// Create decryptor
	decryptor := crypto.NewDecryptor(vaultClient, nil)

	// Decrypt the file
	decryptedFile := filepath.Join(tmpDir, "decrypted.txt")
	err = decryptor.DecryptFile(ctx, encryptedFile, keyFile, decryptedFile, nil)
	require.NoError(t, err)

	// Verify checksum matches
	decryptedChecksum, err := crypto.CalculateChecksum(decryptedFile)
	require.NoError(t, err)
	assert.Equal(t, originalChecksum, decryptedChecksum)
}

// TestLargeFileEncryption tests encryption of files larger than 1MB
func TestLargeFileEncryption(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check for required environment variables
	vaultAddr := os.Getenv("VAULT_ADDR")
	vaultToken := os.Getenv("VAULT_TOKEN")

	if vaultAddr == "" || vaultToken == "" {
		t.Skip("Skipping integration test: VAULT_ADDR or VAULT_TOKEN not set")
	}

	// Set Vault token for API client
	os.Setenv("VAULT_TOKEN", vaultToken)

	// Create temporary directories
	tmpDir := t.TempDir()

	// Create a large test file (2MB)
	testFile := filepath.Join(tmpDir, "large.bin")
	largeData := make([]byte, 2*1024*1024) // 2MB
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	require.NoError(t, os.WriteFile(testFile, largeData, 0644))

	// Initialize Vault client
	vaultClient, err := vault.NewClient(&vault.Config{
		AgentAddress: vaultAddr,
		TransitMount: "transit",
		KeyName:      "file-encryption-key",
		Timeout:      30 * time.Second,
	})
	require.NoError(t, err)

	// Create encryptor
	encryptor := crypto.NewEncryptor(vaultClient, nil)

	// Encrypt the file
	encryptedFile := filepath.Join(tmpDir, "large.bin.enc")
	keyFile := filepath.Join(tmpDir, "large.bin.key")
	ctx := context.Background()
	encryptedKey, err := encryptor.EncryptFile(ctx, testFile, encryptedFile, nil)
	require.NoError(t, err)

	// Save the encrypted key to file
	require.NoError(t, os.WriteFile(keyFile, []byte(encryptedKey), 0644))

	// Create decryptor
	decryptor := crypto.NewDecryptor(vaultClient, nil)

	// Decrypt the file
	decryptedFile := filepath.Join(tmpDir, "large-decrypted.bin")
	err = decryptor.DecryptFile(ctx, encryptedFile, keyFile, decryptedFile, nil)
	require.NoError(t, err)

	// Verify decrypted content matches original
	decryptedData, err := os.ReadFile(decryptedFile)
	require.NoError(t, err)
	assert.Equal(t, largeData, decryptedData)
}

// TestConfigValidation tests configuration validation
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *config.Config
		shouldErr bool
	}{
		{
			name: "valid config",
			cfg: &config.Config{
				Vault: config.VaultConfig{
					AgentAddress:   "http://127.0.0.1:8200",
					TransitMount:   "transit",
					KeyName:        "file-encryption-key",
					RequestTimeout: config.Duration(30 * time.Second),
				},
				Encryption: config.EncryptionConfig{
					SourceDir:          t.TempDir(),
					DestDir:            t.TempDir(),
					SourceFileBehavior: "archive",
					CalculateChecksum:  true,
				},
				Queue: config.QueueConfig{
					StatePath:         filepath.Join(t.TempDir(), "queue.json"),
					MaxRetries:        3,
					BaseDelay:         config.Duration(1 * time.Second),
					MaxDelay:          config.Duration(5 * time.Minute),
					StabilityDuration: config.Duration(1 * time.Second),
				},
				Logging: config.LoggingConfig{
					Level:  "info",
					Output: "stdout",
					Format: "text",
				},
			},
			shouldErr: false,
		},
		{
			name: "missing vault address",
			cfg: &config.Config{
				Vault: config.VaultConfig{
					TransitMount: "transit",
					KeyName:      "file-encryption-key",
				},
			},
			shouldErr: true,
		},
		{
			name: "invalid log level",
			cfg: &config.Config{
				Vault: config.VaultConfig{
					AgentAddress: "http://127.0.0.1:8200",
					TransitMount: "transit",
					KeyName:      "file-encryption-key",
				},
				Logging: config.LoggingConfig{
					Level: "invalid",
				},
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
