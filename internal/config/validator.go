package config

import (
	"fmt"
	"os"
	"strings"
)

// ValidationFunc is a function that validates a config and returns an error
type ValidationFunc func(*Config) error

// validationRules defines all validation rules to be applied to the configuration
var validationRules = []ValidationFunc{
	validateVaultAddress,
	validateVaultTransitMount,
	validateVaultKeyName,
	validateEncryptionSourceDir,
	validateEncryptionDestDir,
	validateEncryptionSourceDirExists,
	validateEncryptionDestDirExists,
	validateEncryptionSourceFileBehavior,
	validateEncryptionChunkSize,
	validateDecryptionIfEnabled,
	validateQueueStatePath,
	validateQueueMaxRetries,
	validateLoggingLevel,
	validateLoggingFormat,
}

// Validate validates the configuration using all validation rules
func (c *Config) Validate() error {
	for _, rule := range validationRules {
		if err := rule(c); err != nil {
			return err
		}
	}
	return nil
}

// Vault validation rules
func validateVaultAddress(c *Config) error {
	if c.Vault.AgentAddress == "" {
		return fmt.Errorf("vault config: agent_address is required")
	}
	return nil
}

func validateVaultTransitMount(c *Config) error {
	if c.Vault.TransitMount == "" {
		return fmt.Errorf("vault config: transit_mount is required")
	}
	return nil
}

func validateVaultKeyName(c *Config) error {
	if c.Vault.KeyName == "" {
		return fmt.Errorf("vault config: key_name is required")
	}
	return nil
}

// Encryption validation rules
func validateEncryptionSourceDir(c *Config) error {
	if c.Encryption.SourceDir == "" {
		return fmt.Errorf("encryption config: source_dir is required")
	}
	return nil
}

func validateEncryptionDestDir(c *Config) error {
	if c.Encryption.DestDir == "" {
		return fmt.Errorf("encryption config: dest_dir is required")
	}
	return nil
}

func validateEncryptionSourceDirExists(c *Config) error {
	if err := ensureDirectoryExists(c.Encryption.SourceDir); err != nil {
		return fmt.Errorf("encryption config: source_dir: %w", err)
	}
	return nil
}

func validateEncryptionDestDirExists(c *Config) error {
	if err := ensureDirectoryExists(c.Encryption.DestDir); err != nil {
		return fmt.Errorf("encryption config: dest_dir: %w", err)
	}
	return nil
}

func validateEncryptionSourceFileBehavior(c *Config) error {
	behavior := strings.ToLower(c.Encryption.SourceFileBehavior)
	if behavior != "archive" && behavior != "delete" && behavior != "keep" {
		return fmt.Errorf("encryption config: source_file_behavior must be 'archive', 'delete', or 'keep', got '%s'", behavior)
	}
	c.Encryption.SourceFileBehavior = behavior
	return nil
}

func validateEncryptionChunkSize(c *Config) error {
	const (
		minChunkSize = 64 * 1000        // 64KB in SI units (humanize uses base 1000)
		maxChunkSize = 10 * 1000 * 1000 // 10MB in SI units
	)

	if c.Encryption.ChunkSize < minChunkSize {
		return fmt.Errorf("encryption config: chunk_size must be >= 64KB, got %s", FormatSize(c.Encryption.ChunkSize))
	}

	if c.Encryption.ChunkSize > maxChunkSize {
		return fmt.Errorf("encryption config: chunk_size must be <= 10MB, got %s", FormatSize(c.Encryption.ChunkSize))
	}

	// Check if it's at least 4KB (AES block alignment consideration)
	if c.Encryption.ChunkSize < 4096 {
		return fmt.Errorf("encryption config: chunk_size must be >= 4KB for AES alignment, got %s", FormatSize(c.Encryption.ChunkSize))
	}

	return nil
}

// Decryption validation rules
func validateDecryptionIfEnabled(c *Config) error {
	if c.Decryption == nil || !c.Decryption.Enabled {
		return nil
	}

	if c.Decryption.SourceDir == "" {
		return fmt.Errorf("decryption config: source_dir is required")
	}

	if c.Decryption.DestDir == "" {
		return fmt.Errorf("decryption config: dest_dir is required")
	}

	if err := ensureDirectoryExists(c.Decryption.SourceDir); err != nil {
		return fmt.Errorf("decryption config: source_dir: %w", err)
	}

	if err := ensureDirectoryExists(c.Decryption.DestDir); err != nil {
		return fmt.Errorf("decryption config: dest_dir: %w", err)
	}

	behavior := strings.ToLower(c.Decryption.SourceFileBehavior)
	if behavior != "archive" && behavior != "delete" && behavior != "keep" {
		return fmt.Errorf("decryption config: source_file_behavior must be 'archive', 'delete', or 'keep', got '%s'", behavior)
	}
	c.Decryption.SourceFileBehavior = behavior

	return nil
}

// Queue validation rules
func validateQueueStatePath(c *Config) error {
	if c.Queue.StatePath == "" {
		return fmt.Errorf("queue config: state_path is required")
	}
	return nil
}

func validateQueueMaxRetries(c *Config) error {
	if c.Queue.MaxRetries < -1 {
		return fmt.Errorf("queue config: max_retries must be >= -1, got %d", c.Queue.MaxRetries)
	}
	return nil
}

// Logging validation rules
func validateLoggingLevel(c *Config) error {
	level := strings.ToLower(c.Logging.Level)
	if level != "debug" && level != "info" && level != "error" {
		return fmt.Errorf("logging config: level must be 'debug', 'info', or 'error', got '%s'", level)
	}
	c.Logging.Level = level
	return nil
}

func validateLoggingFormat(c *Config) error {
	format := strings.ToLower(c.Logging.Format)
	if format != "text" && format != "json" {
		return fmt.Errorf("logging config: format must be 'text' or 'json', got '%s'", format)
	}
	c.Logging.Format = format
	return nil
}

// Helper functions
func ensureDirectoryExists(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0750); err != nil { // #nosec G301 - configurable directory path
			return fmt.Errorf("failed to create directory: %w", err)
		}
		return nil
	}

	if err != nil {
		return err
	}

	if !info.IsDir() {
		return fmt.Errorf("path exists but is not a directory")
	}

	return nil
}
