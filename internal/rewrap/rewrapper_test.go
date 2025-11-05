package rewrap

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gitrgoliveira/vault_file_encryption/internal/logger"
	"github.com/gitrgoliveira/vault_file_encryption/internal/vault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRewrapper(t *testing.T) {
	log, err := logger.New("info", "stderr")
	require.NoError(t, err)

	tests := []struct {
		name        string
		options     RewrapOptions
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid options",
			options: RewrapOptions{
				VaultClient:  &vault.Client{},
				MinVersion:   3,
				Logger:       log,
				CreateBackup: true,
			},
			expectError: false,
		},
		{
			name: "nil vault client",
			options: RewrapOptions{
				MinVersion: 3,
				Logger:     log,
			},
			expectError: true,
			errorMsg:    "vault client is required",
		},
		{
			name: "invalid min version",
			options: RewrapOptions{
				VaultClient: &vault.Client{},
				MinVersion:  0,
				Logger:      log,
			},
			expectError: true,
			errorMsg:    "min_version must be >= 1",
		},
		{
			name: "nil logger",
			options: RewrapOptions{
				VaultClient: &vault.Client{},
				MinVersion:  3,
				Logger:      nil,
			},
			expectError: true,
			errorMsg:    "logger is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rewrapper, err := NewRewrapper(tt.options)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, rewrapper)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, rewrapper)
				assert.NotNil(t, rewrapper.backupManager)
			}
		})
	}
}

func TestRewrapper_RewrapFile(t *testing.T) {
	// Create mock Vault server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/transit/rewrap/test-key" {
			// Simulate rewrap: v1 -> v3
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintln(w, `{"data": {"ciphertext": "vault:v3:newencryptedkey123"}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// Create Vault client
	vaultClient, err := vault.NewClient(&vault.Config{
		AgentAddress: server.URL,
		TransitMount: "transit",
		KeyName:      "test-key",
	})
	require.NoError(t, err)

	log, err := logger.New("info", "stderr")
	require.NoError(t, err)
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		setup       func() (string, RewrapOptions)
		verify      func(*testing.T, *vault.RewrapResult, error)
		expectError bool
	}{
		{
			name: "successful rewrap",
			setup: func() (string, RewrapOptions) {
				keyFile := filepath.Join(tmpDir, "test1.key")
				require.NoError(t, os.WriteFile(keyFile, []byte("vault:v1:oldencryptedkey"), 0644))

				return keyFile, RewrapOptions{
					VaultClient:  vaultClient,
					MinVersion:   3,
					Logger:       log,
					CreateBackup: true,
				}
			},
			verify: func(t *testing.T, result *vault.RewrapResult, err error) {
				require.NoError(t, err)
				assert.Equal(t, 1, result.OldVersion)
				assert.Equal(t, 3, result.NewVersion)
				assert.Equal(t, "vault:v1:oldencryptedkey", result.OldCiphertext)
				assert.Equal(t, "vault:v3:newencryptedkey123", result.NewCiphertext)
				assert.True(t, result.BackupCreated)

				// Verify backup exists
				backupPath := result.FilePath + ".bak"
				content, err := os.ReadFile(backupPath)
				require.NoError(t, err)
				assert.Equal(t, "vault:v1:oldencryptedkey", string(content))

				// Verify file was updated
				newContent, err := os.ReadFile(result.FilePath)
				require.NoError(t, err)
				assert.Equal(t, "vault:v3:newencryptedkey123", string(newContent))
			},
		},
		{
			name: "file already at minimum version",
			setup: func() (string, RewrapOptions) {
				keyFile := filepath.Join(tmpDir, "test2.key")
				require.NoError(t, os.WriteFile(keyFile, []byte("vault:v3:alreadynew"), 0644))

				return keyFile, RewrapOptions{
					VaultClient:  vaultClient,
					MinVersion:   3,
					Logger:       log,
					CreateBackup: false,
				}
			},
			verify: func(t *testing.T, result *vault.RewrapResult, err error) {
				require.NoError(t, err)
				assert.Equal(t, 3, result.OldVersion)
				assert.Equal(t, 0, result.NewVersion) // Not rewrapped
				assert.False(t, result.BackupCreated)

				// Verify file unchanged
				content, err := os.ReadFile(result.FilePath)
				require.NoError(t, err)
				assert.Equal(t, "vault:v3:alreadynew", string(content))
			},
		},
		{
			name: "dry-run mode",
			setup: func() (string, RewrapOptions) {
				keyFile := filepath.Join(tmpDir, "test3.key")
				require.NoError(t, os.WriteFile(keyFile, []byte("vault:v1:oldkey"), 0644))

				return keyFile, RewrapOptions{
					VaultClient:  vaultClient,
					MinVersion:   3,
					DryRun:       true,
					Logger:       log,
					CreateBackup: false,
				}
			},
			verify: func(t *testing.T, result *vault.RewrapResult, err error) {
				require.NoError(t, err)
				assert.Equal(t, 1, result.OldVersion)
				assert.Equal(t, 0, result.NewVersion) // Not rewrapped due to dry-run

				// Verify file unchanged
				content, err := os.ReadFile(result.FilePath)
				require.NoError(t, err)
				assert.Equal(t, "vault:v1:oldkey", string(content))
			},
		},
		{
			name: "non-existent file",
			setup: func() (string, RewrapOptions) {
				keyFile := filepath.Join(tmpDir, "nonexistent.key")

				return keyFile, RewrapOptions{
					VaultClient: vaultClient,
					MinVersion:  3,
					Logger:      log,
				}
			},
			verify: func(t *testing.T, result *vault.RewrapResult, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to read key file")
				assert.NotNil(t, result.Error)
			},
		},
		{
			name: "backup without rewrap",
			setup: func() (string, RewrapOptions) {
				keyFile := filepath.Join(tmpDir, "test4.key")
				require.NoError(t, os.WriteFile(keyFile, []byte("vault:v5:highversion"), 0644))

				return keyFile, RewrapOptions{
					VaultClient:  vaultClient,
					MinVersion:   3,
					Logger:       log,
					CreateBackup: true,
				}
			},
			verify: func(t *testing.T, result *vault.RewrapResult, err error) {
				require.NoError(t, err)
				assert.Equal(t, 5, result.OldVersion)
				assert.False(t, result.BackupCreated) // No backup if no rewrap needed
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyFile, options := tt.setup()
			rewrapper, err := NewRewrapper(options)
			require.NoError(t, err)

			ctx := context.Background()
			result, err := rewrapper.RewrapFile(ctx, keyFile)

			tt.verify(t, result, err)
		})
	}
}

func TestRewrapper_RewrapBatch(t *testing.T) {
	// Create mock Vault server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/transit/rewrap/test-key" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintln(w, `{"data": {"ciphertext": "vault:v3:rewrapped"}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	vaultClient, err := vault.NewClient(&vault.Config{
		AgentAddress: server.URL,
		TransitMount: "transit",
		KeyName:      "test-key",
	})
	require.NoError(t, err)

	log, err := logger.New("info", "stderr")
	require.NoError(t, err)
	tmpDir := t.TempDir()

	t.Run("batch rewrap multiple files", func(t *testing.T) {
		// Create test files
		keyFiles := []string{}
		for i := 1; i <= 5; i++ {
			keyFile := filepath.Join(tmpDir, fmt.Sprintf("batch%d.key", i))
			require.NoError(t, os.WriteFile(keyFile, []byte("vault:v1:old"), 0644))
			keyFiles = append(keyFiles, keyFile)
		}

		rewrapper, err := NewRewrapper(RewrapOptions{
			VaultClient:  vaultClient,
			MinVersion:   3,
			Logger:       log,
			CreateBackup: true,
		})
		require.NoError(t, err)

		ctx := context.Background()
		results, err := rewrapper.RewrapBatch(ctx, keyFiles)

		require.NoError(t, err)
		assert.Equal(t, 5, len(results))

		// Verify all succeeded
		for i, result := range results {
			assert.NoError(t, result.Error, "File %d should succeed", i)
			assert.Equal(t, 1, result.OldVersion)
			assert.Equal(t, 3, result.NewVersion)
			assert.True(t, result.BackupCreated)
		}
	})

	t.Run("batch with context cancellation", func(t *testing.T) {
		// Create many files
		keyFiles := []string{}
		for i := 1; i <= 10; i++ {
			keyFile := filepath.Join(tmpDir, fmt.Sprintf("cancel%d.key", i))
			require.NoError(t, os.WriteFile(keyFile, []byte("vault:v1:old"), 0644))
			keyFiles = append(keyFiles, keyFile)
		}

		rewrapper, err := NewRewrapper(RewrapOptions{
			VaultClient: vaultClient,
			MinVersion:  3,
			Logger:      log,
		})
		require.NoError(t, err)

		// Create context that cancels immediately
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		results, err := rewrapper.RewrapBatch(ctx, keyFiles)

		// Should return context error
		require.Error(t, err)
		assert.Equal(t, context.Canceled, err)
		// Some files may have been processed before cancellation
		assert.LessOrEqual(t, len(results), len(keyFiles))
	})
}

func TestRewrapper_writeKeyFileAtomic(t *testing.T) {
	log, err := logger.New("info", "stderr")
	require.NoError(t, err)
	tmpDir := t.TempDir()

	// Need a minimal rewrapper just to test this method
	rewrapper := &Rewrapper{
		options: RewrapOptions{
			Logger: log,
		},
	}

	t.Run("successful atomic write", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "atomic.key")
		data := []byte("new encrypted content")

		err := rewrapper.writeKeyFileAtomic(filePath, data)
		require.NoError(t, err)

		// Verify content
		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, data, content)
	})

	t.Run("overwrite existing file", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "overwrite.key")
		require.NoError(t, os.WriteFile(filePath, []byte("old content"), 0644))

		newData := []byte("new content")
		err := rewrapper.writeKeyFileAtomic(filePath, newData)
		require.NoError(t, err)

		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, newData, content)
	})
}
