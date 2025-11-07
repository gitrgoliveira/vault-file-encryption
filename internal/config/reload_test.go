package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.hcl")

	// Use ToSlash to avoid Windows path escaping issues in HCL
	hclContent := `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key"
}

encryption {
  source_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "source")) + `"
  dest_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "dest")) + `"
  source_file_behavior = "archive"
}

queue {
  state_path = "` + filepath.ToSlash(filepath.Join(tmpDir, "queue.json")) + `"
}

logging {
  level = "info"
  format = "text"
}
`

	err := os.WriteFile(configPath, []byte(hclContent), 0644)
	require.NoError(t, err)

	mgr, err := NewManager(configPath)
	require.NoError(t, err)
	require.NotNil(t, mgr)

	cfg := mgr.Get()
	assert.Equal(t, "http://127.0.0.1:8200", cfg.Vault.AgentAddress)
	assert.Equal(t, "transit", cfg.Vault.TransitMount)
}

func TestNewManager_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.hcl")

	// Missing required fields
	hclContent := `
vault {
  agent_address = "http://127.0.0.1:8200"
}
`

	err := os.WriteFile(configPath, []byte(hclContent), 0644)
	require.NoError(t, err)

	mgr, err := NewManager(configPath)
	assert.Error(t, err)
	assert.Nil(t, mgr)
	// Error could be from parsing or validation
	assert.True(t, err != nil)
}

func TestManager_Get(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.hcl")

	// Use ToSlash to avoid Windows path escaping issues in HCL
	hclContent := `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key"
}

encryption {
  source_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "source")) + `"
  dest_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "dest")) + `"
  source_file_behavior = "archive"
}

queue {
  state_path = "` + filepath.ToSlash(filepath.Join(tmpDir, "queue.json")) + `"
}

logging {
  level = "info"
  format = "text"
}
`

	err := os.WriteFile(configPath, []byte(hclContent), 0644)
	require.NoError(t, err)

	mgr, err := NewManager(configPath)
	require.NoError(t, err)

	// Get should return the same config multiple times
	cfg1 := mgr.Get()
	cfg2 := mgr.Get()
	assert.Equal(t, cfg1, cfg2)
}

func TestManager_Reload(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.hcl")

	// Use ToSlash to avoid Windows path escaping issues in HCL
	hclContent1 := `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key-1"
}

encryption {
  source_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "source")) + `"
  dest_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "dest")) + `"
  source_file_behavior = "archive"
}

queue {
  state_path = "` + filepath.ToSlash(filepath.Join(tmpDir, "queue.json")) + `"
}

logging {
  level = "info"
  format = "text"
}
`

	err := os.WriteFile(configPath, []byte(hclContent1), 0644)
	require.NoError(t, err)

	mgr, err := NewManager(configPath)
	require.NoError(t, err)

	cfg := mgr.Get()
	assert.Equal(t, "test-key-1", cfg.Vault.KeyName)

	// Update config file
	hclContent2 := `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key-2"
}

encryption {
  source_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "source")) + `"
  dest_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "dest")) + `"
  source_file_behavior = "archive"
}

queue {
  state_path = "` + filepath.ToSlash(filepath.Join(tmpDir, "queue.json")) + `"
}

logging {
  level = "debug"
  format = "text"
}
`

	err = os.WriteFile(configPath, []byte(hclContent2), 0644)
	require.NoError(t, err)

	// Reload config
	err = mgr.Reload()
	require.NoError(t, err)

	cfg = mgr.Get()
	assert.Equal(t, "test-key-2", cfg.Vault.KeyName)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestManager_Reload_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.hcl")

	// Use ToSlash to avoid Windows path escaping issues in HCL
	hclContent1 := `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key"
}

encryption {
  source_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "source")) + `"
  dest_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "dest")) + `"
  source_file_behavior = "archive"
}

queue {
  state_path = "` + filepath.ToSlash(filepath.Join(tmpDir, "queue.json")) + `"
}

logging {
  level = "info"
  format = "text"
}
`

	err := os.WriteFile(configPath, []byte(hclContent1), 0644)
	require.NoError(t, err)

	mgr, err := NewManager(configPath)
	require.NoError(t, err)

	// Update with invalid config
	hclContent2 := `
vault {
  agent_address = "http://127.0.0.1:8200"
  # Missing required fields
}
`

	err = os.WriteFile(configPath, []byte(hclContent2), 0644)
	require.NoError(t, err)

	// Reload should fail
	err = mgr.Reload()
	assert.Error(t, err)

	// Original config should still be valid
	cfg := mgr.Get()
	assert.Equal(t, "test-key", cfg.Vault.KeyName)
}

func TestManager_OnReload(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.hcl")

	// Use ToSlash to avoid Windows path escaping issues in HCL
	hclContent1 := `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key-1"
}

encryption {
  source_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "source")) + `"
  dest_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "dest")) + `"
  source_file_behavior = "archive"
}

queue {
  state_path = "` + filepath.ToSlash(filepath.Join(tmpDir, "queue.json")) + `"
}

logging {
  level = "info"
  format = "text"
}
`

	err := os.WriteFile(configPath, []byte(hclContent1), 0644)
	require.NoError(t, err)

	mgr, err := NewManager(configPath)
	require.NoError(t, err)

	// Register callback
	var callbackCalled bool
	var callbackConfig *Config
	mgr.OnReload(func(cfg *Config) {
		callbackCalled = true
		callbackConfig = cfg
	})

	// Update config file
	hclContent2 := `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key-2"
}

encryption {
  source_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "source")) + `"
  dest_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "dest")) + `"
  source_file_behavior = "archive"
}

queue {
  state_path = "` + filepath.ToSlash(filepath.Join(tmpDir, "queue.json")) + `"
}

logging {
  level = "info"
  format = "text"
}
`

	err = os.WriteFile(configPath, []byte(hclContent2), 0644)
	require.NoError(t, err)

	// Reload config
	err = mgr.Reload()
	require.NoError(t, err)

	// Callback should have been called
	assert.True(t, callbackCalled)
	assert.NotNil(t, callbackConfig)
	assert.Equal(t, "test-key-2", callbackConfig.Vault.KeyName)
}

func TestManager_MultipleCallbacks(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.hcl")

	// Use ToSlash to avoid Windows path escaping issues in HCL
	hclContent1 := `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key-1"
}

encryption {
  source_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "source")) + `"
  dest_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "dest")) + `"
  source_file_behavior = "archive"
}

queue {
  state_path = "` + filepath.ToSlash(filepath.Join(tmpDir, "queue.json")) + `"
}

logging {
  level = "info"
  format = "text"
}
`

	err := os.WriteFile(configPath, []byte(hclContent1), 0644)
	require.NoError(t, err)

	mgr, err := NewManager(configPath)
	require.NoError(t, err)

	// Register multiple callbacks
	var callback1Called, callback2Called bool
	mgr.OnReload(func(cfg *Config) {
		callback1Called = true
	})
	mgr.OnReload(func(cfg *Config) {
		callback2Called = true
	})

	// Update config file
	hclContent2 := `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key-2"
}

encryption {
  source_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "source")) + `"
  dest_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "dest")) + `"
  source_file_behavior = "archive"
}

queue {
  state_path = "` + filepath.ToSlash(filepath.Join(tmpDir, "queue.json")) + `"
}

logging {
  level = "info"
  format = "text"
}
`

	err = os.WriteFile(configPath, []byte(hclContent2), 0644)
	require.NoError(t, err)

	// Reload config
	err = mgr.Reload()
	require.NoError(t, err)

	// Both callbacks should have been called
	assert.True(t, callback1Called)
	assert.True(t, callback2Called)
}

func TestManager_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.hcl")

	// Use ToSlash to avoid Windows path escaping issues in HCL
	hclContent := `
vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "test-key"
}

encryption {
  source_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "source")) + `"
  dest_dir = "` + filepath.ToSlash(filepath.Join(tmpDir, "dest")) + `"
  source_file_behavior = "archive"
}

queue {
  state_path = "` + filepath.ToSlash(filepath.Join(tmpDir, "queue.json")) + `"
}

logging {
  level = "info"
  format = "text"
}
`

	err := os.WriteFile(configPath, []byte(hclContent), 0644)
	require.NoError(t, err)

	mgr, err := NewManager(configPath)
	require.NoError(t, err)

	// Simulate concurrent reads
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				cfg := mgr.Get()
				assert.NotNil(t, cfg)
				time.Sleep(time.Microsecond)
			}
		}()
	}

	wg.Wait()
}
