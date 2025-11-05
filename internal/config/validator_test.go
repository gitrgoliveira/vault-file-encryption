package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	destDir := filepath.Join(tmpDir, "dest")

	cfg := &Config{
		Vault: VaultConfig{
			AgentAddress: "http://127.0.0.1:8200",
			TransitMount: "transit",
			KeyName:      "test-key",
		},
		Encryption: EncryptionConfig{
			SourceDir:          sourceDir,
			DestDir:            destDir,
			SourceFileBehavior: "archive",
			ChunkSize:          1024 * 1024, // 1MB
		},
		Queue: QueueConfig{
			StatePath:  filepath.Join(tmpDir, "queue.json"),
			MaxRetries: 3,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}

	err := cfg.Validate()
	assert.NoError(t, err)

	// Directories should have been created
	assert.DirExists(t, sourceDir)
	assert.DirExists(t, destDir)
}

func TestValidate_MissingVaultAddress(t *testing.T) {
	cfg := &Config{
		Vault: VaultConfig{
			TransitMount: "transit",
			KeyName:      "test-key",
		},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent_address is required")
}

func TestValidate_MissingTransitMount(t *testing.T) {
	cfg := &Config{
		Vault: VaultConfig{
			AgentAddress: "http://127.0.0.1:8200",
			KeyName:      "test-key",
		},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transit_mount is required")
}

func TestValidate_MissingKeyName(t *testing.T) {
	cfg := &Config{
		Vault: VaultConfig{
			AgentAddress: "http://127.0.0.1:8200",
			TransitMount: "transit",
		},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key_name is required")
}

func TestValidate_MissingEncryptionSourceDir(t *testing.T) {
	cfg := &Config{
		Vault: VaultConfig{
			AgentAddress: "http://127.0.0.1:8200",
			TransitMount: "transit",
			KeyName:      "test-key",
		},
		Encryption: EncryptionConfig{
			DestDir: "/tmp/dest",
		},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source_dir is required")
}

func TestValidate_InvalidSourceFileBehavior(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Vault: VaultConfig{
			AgentAddress: "http://127.0.0.1:8200",
			TransitMount: "transit",
			KeyName:      "test-key",
		},
		Encryption: EncryptionConfig{
			SourceDir:          filepath.Join(tmpDir, "source"),
			DestDir:            filepath.Join(tmpDir, "dest"),
			SourceFileBehavior: "invalid",
		},
		Queue: QueueConfig{
			StatePath: filepath.Join(tmpDir, "queue.json"),
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source_file_behavior must be")
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Vault: VaultConfig{
			AgentAddress: "http://127.0.0.1:8200",
			TransitMount: "transit",
			KeyName:      "test-key",
		},
		Encryption: EncryptionConfig{
			SourceDir:          filepath.Join(tmpDir, "source"),
			DestDir:            filepath.Join(tmpDir, "dest"),
			SourceFileBehavior: "archive",
			ChunkSize:          1024 * 1024, // 1MB
		},
		Queue: QueueConfig{
			StatePath: filepath.Join(tmpDir, "queue.json"),
		},
		Logging: LoggingConfig{
			Level:  "invalid",
			Format: "text",
		},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "level must be")
}

func TestValidate_InvalidLogFormat(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Vault: VaultConfig{
			AgentAddress: "http://127.0.0.1:8200",
			TransitMount: "transit",
			KeyName:      "test-key",
		},
		Encryption: EncryptionConfig{
			SourceDir:          filepath.Join(tmpDir, "source"),
			DestDir:            filepath.Join(tmpDir, "dest"),
			SourceFileBehavior: "archive",
			ChunkSize:          1024 * 1024, // 1MB
		},
		Queue: QueueConfig{
			StatePath: filepath.Join(tmpDir, "queue.json"),
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "invalid",
		},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "format must be")
}

func TestValidate_NegativeMaxRetries(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Vault: VaultConfig{
			AgentAddress: "http://127.0.0.1:8200",
			TransitMount: "transit",
			KeyName:      "test-key",
		},
		Encryption: EncryptionConfig{
			SourceDir:          filepath.Join(tmpDir, "source"),
			DestDir:            filepath.Join(tmpDir, "dest"),
			SourceFileBehavior: "archive",
			ChunkSize:          1024 * 1024, // 1MB
		},
		Queue: QueueConfig{
			StatePath:  filepath.Join(tmpDir, "queue.json"),
			MaxRetries: -2,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_retries must be >= -1")
}

func TestValidate_WithDecryption(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Vault: VaultConfig{
			AgentAddress: "http://127.0.0.1:8200",
			TransitMount: "transit",
			KeyName:      "test-key",
		},
		Encryption: EncryptionConfig{
			SourceDir:          filepath.Join(tmpDir, "enc-source"),
			DestDir:            filepath.Join(tmpDir, "enc-dest"),
			SourceFileBehavior: "archive",
			ChunkSize:          1024 * 1024, // 1MB
		},
		Decryption: &DecryptionConfig{
			Enabled:            true,
			SourceDir:          filepath.Join(tmpDir, "dec-source"),
			DestDir:            filepath.Join(tmpDir, "dec-dest"),
			SourceFileBehavior: "delete",
		},
		Queue: QueueConfig{
			StatePath: filepath.Join(tmpDir, "queue.json"),
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}

	err := cfg.Validate()
	assert.NoError(t, err)

	// All directories should have been created
	assert.DirExists(t, filepath.Join(tmpDir, "enc-source"))
	assert.DirExists(t, filepath.Join(tmpDir, "enc-dest"))
	assert.DirExists(t, filepath.Join(tmpDir, "dec-source"))
	assert.DirExists(t, filepath.Join(tmpDir, "dec-dest"))
}

func TestValidate_DecryptionDisabled(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Vault: VaultConfig{
			AgentAddress: "http://127.0.0.1:8200",
			TransitMount: "transit",
			KeyName:      "test-key",
		},
		Encryption: EncryptionConfig{
			SourceDir:          filepath.Join(tmpDir, "source"),
			DestDir:            filepath.Join(tmpDir, "dest"),
			SourceFileBehavior: "archive",
			ChunkSize:          1024 * 1024, // 1MB
		},
		Decryption: &DecryptionConfig{
			Enabled: false,
			// Missing required fields shouldn't matter when disabled
		},
		Queue: QueueConfig{
			StatePath: filepath.Join(tmpDir, "queue.json"),
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}

	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestEnsureDirectoryExists_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "new-dir")

	err := ensureDirectoryExists(newDir)
	assert.NoError(t, err)
	assert.DirExists(t, newDir)
}

func TestEnsureDirectoryExists_ExistingDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	err := ensureDirectoryExists(tmpDir)
	assert.NoError(t, err)
}

func TestEnsureDirectoryExists_PathIsFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.txt")

	err := os.WriteFile(filePath, []byte("test"), 0644)
	require.NoError(t, err)

	err = ensureDirectoryExists(filePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path exists but is not a directory")
}
