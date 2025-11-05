package rewrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBackupManager(t *testing.T) {
	tests := []struct {
		name           string
		options        BackupOptions
		expectedSuffix string
	}{
		{
			name: "default suffix",
			options: BackupOptions{
				Enabled: true,
			},
			expectedSuffix: ".bak",
		},
		{
			name: "custom suffix",
			options: BackupOptions{
				Enabled: true,
				Suffix:  ".backup",
			},
			expectedSuffix: ".backup",
		},
		{
			name: "disabled",
			options: BackupOptions{
				Enabled: false,
			},
			expectedSuffix: ".bak",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewBackupManager(tt.options)
			require.NotNil(t, manager)
			assert.Equal(t, tt.options.Enabled, manager.options.Enabled)
			assert.Equal(t, tt.expectedSuffix, manager.options.Suffix)
		})
	}
}

func TestBackupManager_CreateBackup(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		setup       func() (string, BackupOptions)
		verify      func(*testing.T, string, string, error)
		expectError bool
	}{
		{
			name: "successful backup creation",
			setup: func() (string, BackupOptions) {
				filePath := filepath.Join(tmpDir, "test.key")
				require.NoError(t, os.WriteFile(filePath, []byte("original content"), 0644))
				return filePath, BackupOptions{Enabled: true}
			},
			verify: func(t *testing.T, filePath, backupPath string, err error) {
				require.NoError(t, err)
				assert.NotEmpty(t, backupPath)
				assert.Equal(t, filePath+".bak", backupPath)

				// Verify backup file exists and has correct content
				content, err := os.ReadFile(backupPath)
				require.NoError(t, err)
				assert.Equal(t, "original content", string(content))

				// Verify original file is unchanged
				originalContent, err := os.ReadFile(filePath)
				require.NoError(t, err)
				assert.Equal(t, "original content", string(originalContent))
			},
		},
		{
			name: "backup disabled",
			setup: func() (string, BackupOptions) {
				filePath := filepath.Join(tmpDir, "test2.key")
				require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))
				return filePath, BackupOptions{Enabled: false}
			},
			verify: func(t *testing.T, filePath, backupPath string, err error) {
				require.NoError(t, err)
				assert.Empty(t, backupPath)

				// Verify no backup file was created
				_, err = os.Stat(filePath + ".bak")
				assert.True(t, os.IsNotExist(err))
			},
		},
		{
			name: "custom backup suffix",
			setup: func() (string, BackupOptions) {
				filePath := filepath.Join(tmpDir, "test3.key")
				require.NoError(t, os.WriteFile(filePath, []byte("data"), 0644))
				return filePath, BackupOptions{Enabled: true, Suffix: ".backup"}
			},
			verify: func(t *testing.T, filePath, backupPath string, err error) {
				require.NoError(t, err)
				assert.Equal(t, filePath+".backup", backupPath)

				// Verify backup exists
				_, err = os.Stat(backupPath)
				require.NoError(t, err)
			},
		},
		{
			name: "overwrite existing backup",
			setup: func() (string, BackupOptions) {
				filePath := filepath.Join(tmpDir, "test4.key")
				require.NoError(t, os.WriteFile(filePath, []byte("new content"), 0644))

				// Create existing backup
				backupPath := filePath + ".bak"
				require.NoError(t, os.WriteFile(backupPath, []byte("old backup"), 0644))

				return filePath, BackupOptions{Enabled: true}
			},
			verify: func(t *testing.T, filePath, backupPath string, err error) {
				require.NoError(t, err)

				// Verify backup was overwritten with new content
				content, err := os.ReadFile(backupPath)
				require.NoError(t, err)
				assert.Equal(t, "new content", string(content))
			},
		},
		{
			name: "non-existent source file",
			setup: func() (string, BackupOptions) {
				filePath := filepath.Join(tmpDir, "nonexistent.key")
				return filePath, BackupOptions{Enabled: true}
			},
			verify: func(t *testing.T, filePath, backupPath string, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "source file not found")
			},
		},
		{
			name: "source is directory",
			setup: func() (string, BackupOptions) {
				dirPath := filepath.Join(tmpDir, "testdir")
				require.NoError(t, os.Mkdir(dirPath, 0755))
				return dirPath, BackupOptions{Enabled: true}
			},
			verify: func(t *testing.T, filePath, backupPath string, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "cannot backup directory")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath, options := tt.setup()
			manager := NewBackupManager(options)

			backupPath, err := manager.CreateBackup(filePath)
			tt.verify(t, filePath, backupPath, err)
		})
	}
}

func TestBackupManager_RestoreBackup(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name   string
		setup  func() (string, *BackupManager)
		verify func(*testing.T, string, error)
	}{
		{
			name: "successful restore",
			setup: func() (string, *BackupManager) {
				filePath := filepath.Join(tmpDir, "test.key")
				backupPath := filePath + ".bak"

				// Create original file
				require.NoError(t, os.WriteFile(filePath, []byte("modified"), 0644))

				// Create backup with original content
				require.NoError(t, os.WriteFile(backupPath, []byte("original"), 0644))

				manager := NewBackupManager(BackupOptions{Enabled: true})
				return filePath, manager
			},
			verify: func(t *testing.T, filePath string, err error) {
				require.NoError(t, err)

				// Verify file was restored
				content, err := os.ReadFile(filePath)
				require.NoError(t, err)
				assert.Equal(t, "original", string(content))
			},
		},
		{
			name: "restore non-existent backup",
			setup: func() (string, *BackupManager) {
				filePath := filepath.Join(tmpDir, "nobackup.key")
				require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

				manager := NewBackupManager(BackupOptions{Enabled: true})
				return filePath, manager
			},
			verify: func(t *testing.T, filePath string, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "backup file not found")
			},
		},
		{
			name: "restore with custom suffix",
			setup: func() (string, *BackupManager) {
				filePath := filepath.Join(tmpDir, "custom.key")
				backupPath := filePath + ".backup"

				require.NoError(t, os.WriteFile(filePath, []byte("current"), 0644))
				require.NoError(t, os.WriteFile(backupPath, []byte("backup data"), 0644))

				manager := NewBackupManager(BackupOptions{Enabled: true, Suffix: ".backup"})
				return filePath, manager
			},
			verify: func(t *testing.T, filePath string, err error) {
				require.NoError(t, err)

				content, err := os.ReadFile(filePath)
				require.NoError(t, err)
				assert.Equal(t, "backup data", string(content))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath, manager := tt.setup()
			err := manager.RestoreBackup(filePath)
			tt.verify(t, filePath, err)
		})
	}
}

func TestBackupManager_RemoveBackup(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name   string
		setup  func() (string, *BackupManager)
		verify func(*testing.T, string, error)
	}{
		{
			name: "remove existing backup",
			setup: func() (string, *BackupManager) {
				filePath := filepath.Join(tmpDir, "test.key")
				backupPath := filePath + ".bak"

				require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))
				require.NoError(t, os.WriteFile(backupPath, []byte("backup"), 0644))

				manager := NewBackupManager(BackupOptions{Enabled: true})
				return filePath, manager
			},
			verify: func(t *testing.T, filePath string, err error) {
				require.NoError(t, err)

				// Verify backup was deleted
				_, err = os.Stat(filePath + ".bak")
				assert.True(t, os.IsNotExist(err))

				// Verify original file still exists
				_, err = os.Stat(filePath)
				require.NoError(t, err)
			},
		},
		{
			name: "remove non-existent backup (no error)",
			setup: func() (string, *BackupManager) {
				filePath := filepath.Join(tmpDir, "nobackup.key")
				require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

				manager := NewBackupManager(BackupOptions{Enabled: true})
				return filePath, manager
			},
			verify: func(t *testing.T, filePath string, err error) {
				require.NoError(t, err) // Should not error if backup doesn't exist
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath, manager := tt.setup()
			err := manager.RemoveBackup(filePath)
			tt.verify(t, filePath, err)
		})
	}
}

func TestBackupManager_BackupExists(t *testing.T) {
	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "test.key")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

	manager := NewBackupManager(BackupOptions{Enabled: true})

	// Backup should not exist initially
	assert.False(t, manager.BackupExists(filePath))

	// Create backup
	backupPath, err := manager.CreateBackup(filePath)
	require.NoError(t, err)
	require.NotEmpty(t, backupPath)

	// Backup should now exist
	assert.True(t, manager.BackupExists(filePath))

	// Remove backup
	require.NoError(t, manager.RemoveBackup(filePath))

	// Backup should not exist after removal
	assert.False(t, manager.BackupExists(filePath))
}

func TestBackupManager_GetBackupPath(t *testing.T) {
	tests := []struct {
		name         string
		options      BackupOptions
		originalPath string
		expectedPath string
	}{
		{
			name:         "default suffix",
			options:      BackupOptions{Enabled: true},
			originalPath: "/data/file.key",
			expectedPath: "/data/file.key.bak",
		},
		{
			name:         "custom suffix",
			options:      BackupOptions{Enabled: true, Suffix: ".backup"},
			originalPath: "/data/file.key",
			expectedPath: "/data/file.key.backup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewBackupManager(tt.options)
			backupPath := manager.GetBackupPath(tt.originalPath)
			assert.Equal(t, tt.expectedPath, backupPath)
		})
	}
}
