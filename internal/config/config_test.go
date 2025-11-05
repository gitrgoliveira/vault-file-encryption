package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromString(t *testing.T) {
	hclContent := `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key"
}

encryption {
  source_dir = "/tmp/source"
  dest_dir = "/tmp/dest"
  source_file_behavior = "archive"
  calculate_checksum = true
}

queue {
  state_path = "/tmp/queue.json"
  max_retries = 5
}

logging {
  level = "debug"
  output = "stdout"
  format = "json"
  audit_log = true
  audit_path = "/tmp/audit.log"
}
`

	cfg, err := LoadFromString("test.hcl", hclContent)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify Vault config
	assert.Equal(t, "http://127.0.0.1:8200", cfg.Vault.AgentAddress)
	assert.Equal(t, "transit", cfg.Vault.TransitMount)
	assert.Equal(t, "test-key", cfg.Vault.KeyName)
	assert.Equal(t, Duration(30*time.Second), cfg.Vault.RequestTimeout) // Default

	// Verify Encryption config
	assert.Equal(t, "/tmp/source", cfg.Encryption.SourceDir)
	assert.Equal(t, "/tmp/dest", cfg.Encryption.DestDir)
	assert.Equal(t, "archive", cfg.Encryption.SourceFileBehavior)
	assert.True(t, cfg.Encryption.CalculateChecksum)

	// Verify Queue config
	assert.Equal(t, "/tmp/queue.json", cfg.Queue.StatePath)
	assert.Equal(t, 5, cfg.Queue.MaxRetries)
	assert.Equal(t, Duration(1*time.Second), cfg.Queue.BaseDelay)         // Default
	assert.Equal(t, Duration(5*time.Minute), cfg.Queue.MaxDelay)          // Default
	assert.Equal(t, Duration(1*time.Second), cfg.Queue.StabilityDuration) // Default

	// Verify Logging config
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "stdout", cfg.Logging.Output)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.True(t, cfg.Logging.AuditLog)
	assert.Equal(t, "/tmp/audit.log", cfg.Logging.AuditPath)
}

func TestLoadFromString_WithDecryption(t *testing.T) {
	hclContent := `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key"
}

encryption {
  source_dir = "/tmp/source"
  dest_dir = "/tmp/dest"
  source_file_behavior = "delete"
}

decryption {
  enabled = true
  source_dir = "/tmp/enc"
  dest_dir = "/tmp/dec"
  source_file_behavior = "keep"
  verify_checksum = true
}

queue {
  state_path = "/tmp/queue.json"
}

logging {
  level = "info"
}
`

	cfg, err := LoadFromString("test.hcl", hclContent)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify Decryption config
	require.NotNil(t, cfg.Decryption)
	assert.True(t, cfg.Decryption.Enabled)
	assert.Equal(t, "/tmp/enc", cfg.Decryption.SourceDir)
	assert.Equal(t, "/tmp/dec", cfg.Decryption.DestDir)
	assert.Equal(t, "keep", cfg.Decryption.SourceFileBehavior)
	assert.True(t, cfg.Decryption.VerifyChecksum)
}

func TestSetDefaults(t *testing.T) {
	cfg := &Config{
		Vault: VaultConfig{
			AgentAddress: "http://127.0.0.1:8200",
			TransitMount: "transit",
			KeyName:      "test-key",
		},
		Encryption: EncryptionConfig{
			SourceDir: "/tmp/source",
			DestDir:   "/tmp/dest",
		},
		Queue: QueueConfig{
			StatePath: "/tmp/queue.json",
		},
		Logging: LoggingConfig{},
	}

	err := cfg.SetDefaults()
	assert.NoError(t, err)

	// Vault defaults
	assert.Equal(t, Duration(30*time.Second), cfg.Vault.RequestTimeout)

	// Encryption defaults
	assert.Equal(t, "archive", cfg.Encryption.SourceFileBehavior)

	// Queue defaults
	assert.Equal(t, 3, cfg.Queue.MaxRetries)
	assert.Equal(t, Duration(1*time.Second), cfg.Queue.BaseDelay)
	assert.Equal(t, Duration(5*time.Minute), cfg.Queue.MaxDelay)
	assert.Equal(t, Duration(1*time.Second), cfg.Queue.StabilityDuration)

	// Logging defaults
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "stdout", cfg.Logging.Output)
	assert.Equal(t, "text", cfg.Logging.Format)
}

func TestSetDefaults_WithDecryption(t *testing.T) {
	cfg := &Config{
		Vault: VaultConfig{
			AgentAddress: "http://127.0.0.1:8200",
			TransitMount: "transit",
			KeyName:      "test-key",
		},
		Encryption: EncryptionConfig{
			SourceDir: "/tmp/source",
			DestDir:   "/tmp/dest",
		},
		Decryption: &DecryptionConfig{
			Enabled:   true,
			SourceDir: "/tmp/enc",
			DestDir:   "/tmp/dec",
		},
		Queue: QueueConfig{
			StatePath: "/tmp/queue.json",
		},
		Logging: LoggingConfig{},
	}

	err := cfg.SetDefaults()
	assert.NoError(t, err)

	// Decryption defaults
	assert.Equal(t, "archive", cfg.Decryption.SourceFileBehavior)
}

func TestSetDefaults_AuditPath(t *testing.T) {
	cfg := &Config{
		Vault: VaultConfig{
			AgentAddress: "http://127.0.0.1:8200",
			TransitMount: "transit",
			KeyName:      "test-key",
		},
		Encryption: EncryptionConfig{
			SourceDir: "/tmp/source",
			DestDir:   "/tmp/dest",
		},
		Queue: QueueConfig{
			StatePath: "/tmp/queue.json",
		},
		Logging: LoggingConfig{
			AuditLog: true,
		},
	}

	err := cfg.SetDefaults()
	assert.NoError(t, err)

	// Audit path should be set when audit_log is true
	assert.Equal(t, "audit.log", cfg.Logging.AuditPath)
}

func TestArchiveDir(t *testing.T) {
	cfg := &Config{
		Encryption: EncryptionConfig{
			SourceDir: "/tmp/source",
		},
		Decryption: &DecryptionConfig{
			SourceDir: "/tmp/enc",
		},
	}

	assert.Equal(t, "/tmp/source/.archive", cfg.ArchiveDir("encrypt"))
	assert.Equal(t, "/tmp/enc/.archive", cfg.ArchiveDir("decrypt"))
}

func TestFailedDir(t *testing.T) {
	cfg := &Config{
		Encryption: EncryptionConfig{
			SourceDir: "/tmp/source",
		},
		Decryption: &DecryptionConfig{
			SourceDir: "/tmp/enc",
		},
	}

	assert.Equal(t, "/tmp/source/.failed", cfg.FailedDir("encrypt"))
	assert.Equal(t, "/tmp/enc/.failed", cfg.FailedDir("decrypt"))
}

func TestDLQDir(t *testing.T) {
	cfg := &Config{
		Encryption: EncryptionConfig{
			SourceDir: "/tmp/source",
		},
		Decryption: &DecryptionConfig{
			SourceDir: "/tmp/enc",
		},
	}

	assert.Equal(t, "/tmp/source/.dlq", cfg.DLQDir("encrypt"))
	assert.Equal(t, "/tmp/enc/.dlq", cfg.DLQDir("decrypt"))
}

func TestLoad(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.hcl")

	hclContent := `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key"
}

encryption {
  source_dir = "/tmp/source"
  dest_dir = "/tmp/dest"
  source_file_behavior = "archive"
}

queue {
  state_path = "/tmp/queue.json"
}

logging {
  level = "info"
}
`

	err := os.WriteFile(configPath, []byte(hclContent), 0644)
	require.NoError(t, err)

	// Load config
	cfg, err := Load(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "http://127.0.0.1:8200", cfg.Vault.AgentAddress)
	assert.Equal(t, "transit", cfg.Vault.TransitMount)
	assert.Equal(t, "test-key", cfg.Vault.KeyName)
}

func TestLoad_NonExistentFile(t *testing.T) {
	cfg, err := Load("/nonexistent/config.hcl")
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "configuration file not found")
}

func TestLoad_InvalidHCL(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.hcl")

	err := os.WriteFile(configPath, []byte("invalid { hcl syntax"), 0644)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "failed to parse configuration")
}

func TestChunkSizeConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		hclContent    string
		expectedSize  int
		expectError   bool
		errorContains string
	}{
		{
			name: "default chunk size (1MB)",
			hclContent: `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key"
}
encryption {
  source_dir = "/tmp/source"
  dest_dir = "/tmp/dest"
  source_file_behavior = "delete"
}
queue {
  state_path = "/tmp/queue.json"
}
logging {
  level = "info"
}
`,
			expectedSize: 1024 * 1024,
			expectError:  false,
		},
		{
			name: "2MB chunk size",
			hclContent: `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key"
}
encryption {
  source_dir = "/tmp/source"
  dest_dir = "/tmp/dest"
  source_file_behavior = "delete"
  chunk_size = "2MB"
}
queue {
  state_path = "/tmp/queue.json"
}
logging {
  level = "info"
}
`,
			expectedSize: 2 * 1000 * 1000, // 2MB in SI units (humanize uses base 1000)
			expectError:  false,
		},
		{
			name: "512KB chunk size",
			hclContent: `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key"
}
encryption {
  source_dir = "/tmp/source"
  dest_dir = "/tmp/dest"
  source_file_behavior = "delete"
  chunk_size = "512KB"
}
queue {
  state_path = "/tmp/queue.json"
}
logging {
  level = "info"
}
`,
			expectedSize: 512 * 1000, // 512KB in SI units
			expectError:  false,
		},
		{
			name: "10MB chunk size (max)",
			hclContent: `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key"
}
encryption {
  source_dir = "/tmp/source"
  dest_dir = "/tmp/dest"
  source_file_behavior = "delete"
  chunk_size = "10MB"
}
queue {
  state_path = "/tmp/queue.json"
}
logging {
  level = "info"
}
`,
			expectedSize: 10 * 1000 * 1000, // 10MB in SI units
			expectError:  false,
		},
		{
			name: "64KB chunk size (min)",
			hclContent: `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key"
}
encryption {
  source_dir = "/tmp/source"
  dest_dir = "/tmp/dest"
  source_file_behavior = "delete"
  chunk_size = "64KB"
}
queue {
  state_path = "/tmp/queue.json"
}
logging {
  level = "info"
}
`,
			expectedSize: 64 * 1000, // 64KB in SI units
			expectError:  false,
		},
		{
			name: "chunk size too small",
			hclContent: `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key"
}
encryption {
  source_dir = "/tmp/source"
  dest_dir = "/tmp/dest"
  source_file_behavior = "delete"
  chunk_size = "32KB"
}
queue {
  state_path = "/tmp/queue.json"
}
logging {
  level = "info"
}
`,
			expectError:   true,
			errorContains: "chunk_size must be >= 64KB",
		},
		{
			name: "chunk size too large",
			hclContent: `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key"
}
encryption {
  source_dir = "/tmp/source"
  dest_dir = "/tmp/dest"
  source_file_behavior = "delete"
  chunk_size = "20MB"
}
queue {
  state_path = "/tmp/queue.json"
}
logging {
  level = "info"
}
`,
			expectError:   true,
			errorContains: "chunk_size must be <= 10MB",
		},
		{
			name: "invalid chunk size format",
			hclContent: `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key"
}
encryption {
  source_dir = "/tmp/source"
  dest_dir = "/tmp/dest"
  source_file_behavior = "delete"
  chunk_size = "invalid"
}
queue {
  state_path = "/tmp/queue.json"
}
logging {
  level = "info"
}
`,
			expectError:   true,
			errorContains: "invalid chunk_size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directories for testing
			tmpDir := t.TempDir()
			sourceDir := filepath.Join(tmpDir, "source")
			destDir := filepath.Join(tmpDir, "dest")

			// Create the directories
			require.NoError(t, os.MkdirAll(sourceDir, 0755))
			require.NoError(t, os.MkdirAll(destDir, 0755))

			// Replace placeholders in HCL content
			hcl := tt.hclContent
			hcl = os.Expand(hcl, func(key string) string {
				switch key {
				case "SOURCE_DIR":
					return sourceDir
				case "DEST_DIR":
					return destDir
				default:
					return ""
				}
			})

			cfg, err := LoadFromString("test.hcl", hcl)

			if tt.expectError {
				// For validation errors, cfg might load successfully but fail validation
				if err == nil && cfg != nil {
					err = cfg.Validate()
				}
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)

			// Validate the config
			err = cfg.Validate()
			require.NoError(t, err)

			assert.Equal(t, tt.expectedSize, cfg.Encryption.ChunkSize,
				"Chunk size should be %d bytes (%s)", tt.expectedSize, FormatSize(tt.expectedSize))
		})
	}
}
