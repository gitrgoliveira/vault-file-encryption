package config

import (
	"fmt"
)

// Config represents the application configuration
type Config struct {
	Vault      VaultConfig       `hcl:"vault,block"`
	Encryption EncryptionConfig  `hcl:"encryption,block"`
	Decryption *DecryptionConfig `hcl:"decryption,block"`
	Queue      QueueConfig       `hcl:"queue,block"`
	Logging    LoggingConfig     `hcl:"logging,block"`
}

// VaultConfig holds Vault-related configuration
type VaultConfig struct {
	AgentAddress   string   `hcl:"agent_address"`
	TransitMount   string   `hcl:"transit_mount"`
	KeyName        string   `hcl:"key_name"`
	RequestTimeout Duration `hcl:"request_timeout,optional"`
}

// EncryptionConfig holds encryption-specific configuration
type EncryptionConfig struct {
	SourceDir          string `hcl:"source_dir"`
	DestDir            string `hcl:"dest_dir"`
	SourceFileBehavior string `hcl:"source_file_behavior"`
	CalculateChecksum  bool   `hcl:"calculate_checksum,optional"`
	FilePattern        string `hcl:"file_pattern,optional"`
	ChunkSizeStr       string `hcl:"chunk_size,optional"`
	ChunkSize          int    // Parsed from ChunkSizeStr
}

// DecryptionConfig holds decryption-specific configuration
type DecryptionConfig struct {
	Enabled            bool   `hcl:"enabled,optional"`
	SourceDir          string `hcl:"source_dir"`
	DestDir            string `hcl:"dest_dir"`
	SourceFileBehavior string `hcl:"source_file_behavior"`
	VerifyChecksum     bool   `hcl:"verify_checksum,optional"`
}

// QueueConfig holds queue-related configuration
type QueueConfig struct {
	StatePath         string   `hcl:"state_path"`
	MaxRetries        int      `hcl:"max_retries,optional"`
	BaseDelay         Duration `hcl:"base_delay,optional"`
	MaxDelay          Duration `hcl:"max_delay,optional"`
	StabilityDuration Duration `hcl:"stability_duration,optional"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level     string `hcl:"level,optional"`
	Output    string `hcl:"output,optional"`
	Format    string `hcl:"format,optional"`
	AuditLog  bool   `hcl:"audit_log,optional"`
	AuditPath string `hcl:"audit_path,optional"`
}

// SetDefaults sets default values for optional fields
func (c *Config) SetDefaults() error {
	// Vault defaults - validation happens in UnmarshalText
	if c.Vault.RequestTimeout == 0 {
		c.Vault.RequestTimeout = Duration(DefaultVaultTimeout)
	}

	// Encryption defaults
	if c.Encryption.SourceFileBehavior == "" {
		c.Encryption.SourceFileBehavior = "archive"
	}

	// Parse chunk size
	if c.Encryption.ChunkSizeStr != "" {
		chunkSize, err := ParseSize(c.Encryption.ChunkSizeStr)
		if err != nil {
			return fmt.Errorf("invalid chunk_size: %w", err)
		}
		c.Encryption.ChunkSize = chunkSize
	}
	if c.Encryption.ChunkSize == 0 {
		c.Encryption.ChunkSize = 1024 * 1024 // Default 1MB
	}

	// Decryption defaults
	if c.Decryption != nil {
		if c.Decryption.SourceFileBehavior == "" {
			c.Decryption.SourceFileBehavior = "archive"
		}
	}

	// Queue defaults - validation happens in UnmarshalText
	if c.Queue.MaxRetries == 0 {
		c.Queue.MaxRetries = DefaultMaxRetries
	}
	if c.Queue.BaseDelay == 0 {
		c.Queue.BaseDelay = Duration(DefaultBaseDelay)
	}
	if c.Queue.MaxDelay == 0 {
		c.Queue.MaxDelay = Duration(DefaultMaxDelay)
	}
	if c.Queue.StabilityDuration == 0 {
		c.Queue.StabilityDuration = Duration(DefaultStabilityDuration)
	}

	// Logging defaults
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Output == "" {
		c.Logging.Output = "stdout"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "text"
	}
	if c.Logging.AuditLog && c.Logging.AuditPath == "" {
		c.Logging.AuditPath = "audit.log"
	}

	return nil
}

// ArchiveDir returns the archive directory path for the given operation
func (c *Config) ArchiveDir(operation string) string {
	if operation == "encrypt" {
		return c.Encryption.SourceDir + "/.archive"
	}
	if c.Decryption != nil {
		return c.Decryption.SourceDir + "/.archive"
	}
	return ""
}

// FailedDir returns the failed directory path for the given operation
func (c *Config) FailedDir(operation string) string {
	if operation == "encrypt" {
		return c.Encryption.SourceDir + "/.failed"
	}
	if c.Decryption != nil {
		return c.Decryption.SourceDir + "/.failed"
	}
	return ""
}

// DLQDir returns the dead letter queue directory path for the given operation
func (c *Config) DLQDir(operation string) string {
	if operation == "encrypt" {
		return c.Encryption.SourceDir + "/.dlq"
	}
	if c.Decryption != nil {
		return c.Decryption.SourceDir + "/.dlq"
	}
	return ""
}
