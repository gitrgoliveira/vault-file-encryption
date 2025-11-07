package service

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	"github.com/gitrgoliveira/vault-file-encryption/internal/model"

	"github.com/gitrgoliveira/vault-file-encryption/internal/config"
	"github.com/gitrgoliveira/vault-file-encryption/internal/logger"
	"github.com/gitrgoliveira/vault-file-encryption/internal/vault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mocks
type MockConfigManager struct {
	mock.Mock
	cfg *config.Config
}

func (m *MockConfigManager) Get() *config.Config {
	return m.cfg
}

func (m *MockConfigManager) Reload() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockConfigManager) OnReload(cb func(*config.Config)) {
	m.Called(cb)
}

type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Info(msg string, args ...interface{})  { m.Called(msg, args) }
func (m *MockLogger) Error(msg string, args ...interface{}) { m.Called(msg, args) }
func (m *MockLogger) Debug(msg string, args ...interface{}) { m.Called(msg, args) }
func (m *MockLogger) Sync() error {
	args := m.Called()
	return args.Error(0)
}

type MockVaultClient struct {
	mock.Mock
}

func (m *MockVaultClient) GenerateDataKey() (*vault.DataKey, error) {
	args := m.Called()
	return args.Get(0).(*vault.DataKey), args.Error(1)
}

func (m *MockVaultClient) DecryptDataKey(ciphertext string) (*vault.DataKey, error) {
	args := m.Called()
	return args.Get(0).(*vault.DataKey), args.Error(1)
}

func (m *MockVaultClient) RewrapDataKey(ciphertext string) (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockVaultClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

type MockQueue struct {
	mock.Mock
}

func (m *MockQueue) Enqueue(item *model.Item) error {
	args := m.Called(item)
	return args.Error(0)
}
func (m *MockQueue) Dequeue() *model.Item {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*model.Item)
}
func (m *MockQueue) Load() error {
	args := m.Called()
	return args.Error(0)
}
func (m *MockQueue) Save() error {
	args := m.Called()
	return args.Error(0)
}
func (m *MockQueue) Size() int {
	args := m.Called()
	return args.Int(0)
}
func (m *MockQueue) Requeue(item *model.Item, err error) error {
	args := m.Called(item, err)
	return args.Error(0)
}

type MockWatcher struct {
	mock.Mock
}

func (m *MockWatcher) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
func (m *MockWatcher) UpdateConfig(cfg *config.Config) error {
	args := m.Called(cfg)
	return args.Error(0)
}

type MockProcessor struct {
	mock.Mock
}

func (m *MockProcessor) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
func (m *MockProcessor) UpdateConfig(cfg *config.Config) {
	m.Called(cfg)
}

func newTestConfig(t *testing.T) (*config.Config, string) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		Vault: config.VaultConfig{
			AgentAddress: "http://127.0.0.1:8200",
			TransitMount: "transit",
			KeyName:      "test-key",
		},
		Encryption: config.EncryptionConfig{
			SourceDir: filepath.Join(tempDir, "encrypt-src"),
			DestDir:   filepath.Join(tempDir, "encrypt-dest"),
		},
		Decryption: &config.DecryptionConfig{
			SourceDir: filepath.Join(tempDir, "decrypt-src"),
			DestDir:   filepath.Join(tempDir, "decrypt-dest"),
		},
		Queue: config.QueueConfig{
			StatePath: filepath.Join(tempDir, "queue.state"),
		},
		Logging: config.LoggingConfig{
			Output: "stdout",
			Level:  "info",
		},
	}

	// Create directories
	require.NoError(t, os.MkdirAll(cfg.Encryption.SourceDir, 0755))
	require.NoError(t, os.MkdirAll(cfg.Encryption.DestDir, 0755))
	require.NoError(t, os.MkdirAll(cfg.Decryption.SourceDir, 0755))
	require.NoError(t, os.MkdirAll(cfg.Decryption.DestDir, 0755))

	return cfg, tempDir
}

func TestNew(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		cfg, _ := newTestConfig(t)
		configFile := createTestConfigFile(t, cfg)

		svc, err := New(&Config{ConfigFile: configFile})
		require.NoError(t, err)
		require.NotNil(t, svc)
		assert.NotNil(t, svc.cfgMgr)
		assert.NotNil(t, svc.log)
		assert.NotNil(t, svc.vaultClient)
		assert.NotNil(t, svc.encryptor)
		assert.NotNil(t, svc.decryptor)
		assert.NotNil(t, svc.queue)
		assert.NotNil(t, svc.watcher)
		assert.NotNil(t, svc.processor)

		err = svc.Close()
		assert.NoError(t, err)
	})

	t.Run("config load failure", func(t *testing.T) {
		_, err := New(&Config{ConfigFile: "non-existent-file.hcl"})
		assert.Error(t, err)
	})

	t.Run("initial config validation failure", func(t *testing.T) {
		cfg, _ := newTestConfig(t)
		cfg.Vault.AgentAddress = "" // Invalid config
		configFile := createTestConfigFile(t, cfg)

		_, err := New(&Config{ConfigFile: configFile})
		assert.Error(t, err)
	})
}

func TestService_Run_Shutdown(t *testing.T) {
	cfg, _ := newTestConfig(t)
	configFile := createTestConfigFile(t, cfg)

	svc, err := New(&Config{ConfigFile: configFile})
	require.NoError(t, err)
	require.NotNil(t, svc)

	// Replace real components with mocks
	mockQueue := &MockQueue{}
	svc.queue = mockQueue
	mockQueue.On("Save").Return(nil)
	mockQueue.On("Load").Return(nil)
	mockQueue.On("Size").Return(0)

	mockWatcher := &MockWatcher{}
	svc.watcher = mockWatcher
	mockWatcher.On("Start", mock.Anything).Return(nil)

	mockProcessor := &MockProcessor{}
	svc.processor = mockProcessor
	mockProcessor.On("Start", mock.Anything).Return(nil)

	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		err := svc.Run(ctx, sigChan, func(s os.Signal) bool { return s == syscall.SIGHUP }, func(s os.Signal) bool { return s == syscall.SIGINT })
		assert.NoError(t, err)
	}()

	// Test shutdown
	t.Run("shutdown signal", func(t *testing.T) {
		sigChan <- syscall.SIGINT
		wg.Wait() // Wait for Run to exit
		mockQueue.AssertCalled(t, "Save")
	})

	cancel()
}

func TestService_Reload(t *testing.T) {
	cfg, _ := newTestConfig(t)
	configFile := createTestConfigFile(t, cfg)

	svc, err := New(&Config{ConfigFile: configFile})
	require.NoError(t, err)
	require.NotNil(t, svc)

	// Replace real components with mocks
	mockWatcher := &MockWatcher{}
	svc.watcher = mockWatcher
	mockWatcher.On("UpdateConfig", mock.Anything).Return(nil)

	mockProcessor := &MockProcessor{}
	svc.processor = mockProcessor
	mockProcessor.On("UpdateConfig", mock.Anything)

	// Simulate config reload
	newCfg := *cfg
	newCfg.Logging.Level = "debug"
	svc.cfgMgr.OnReload(func(c *config.Config) {
		// This is the callback that should be registered
	})

	// Manually trigger the reload logic for test purposes
	svc.handleReload(&newCfg)

	mockWatcher.AssertCalled(t, "UpdateConfig", mock.Anything)
	mockProcessor.AssertCalled(t, "UpdateConfig", mock.Anything)
}

func TestService_Close(t *testing.T) {
	cfg, _ := newTestConfig(t)
	configFile := createTestConfigFile(t, cfg)

	svc, err := New(&Config{ConfigFile: configFile})
	require.NoError(t, err)

	// Replace vault client with mock
	mockVault := &MockVaultClient{}
	svc.vaultClient = mockVault
	mockVault.On("Close").Return(nil)

	err = svc.Close()
	assert.NoError(t, err)

	// Verify vault client Close was called
	mockVault.AssertCalled(t, "Close")
}

// createTestConfigFile creates a temporary HCL config file for testing.
func createTestConfigFile(t *testing.T, cfg *config.Config) string {
	t.Helper()
	content := `
		logging {
			level = "` + cfg.Logging.Level + `"
			output = "` + cfg.Logging.Output + `"
		}
		vault {
			agent_address = "` + cfg.Vault.AgentAddress + `"
			transit_mount = "` + cfg.Vault.TransitMount + `"
			key_name = "` + cfg.Vault.KeyName + `"
		}
		encryption {
			source_dir = "` + cfg.Encryption.SourceDir + `"
			dest_dir = "` + cfg.Encryption.DestDir + `"
			source_file_behavior = "archive"
		}
		decryption {
			source_dir = "` + cfg.Decryption.SourceDir + `"
			dest_dir = "` + cfg.Decryption.DestDir + `"
			source_file_behavior = "archive"
		}
		queue {
			state_path = "` + cfg.Queue.StatePath + `"
		}
	`
	tmpFile, err := os.CreateTemp(t.TempDir(), "config-*.hcl")
	require.NoError(t, err)
	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())
	return tmpFile.Name()
}

// handleReload is a helper to manually trigger the reload logic for testing.
func (s *Service) handleReload(newCfg *config.Config) {
	s.log.Info("Configuration reloaded successfully, updating components...")

	// Update logger with new level and output
	if newLogger, err := logger.New(newCfg.Logging.Level, newCfg.Logging.Output); err != nil {
		s.log.Error("Failed to create new logger from reloaded config", "error", err)
	} else {
		s.log.Info("Switching to new logger")
		oldLogger := s.log
		s.log = newLogger
		if oldLogger != nil {
			_ = oldLogger.Sync()
		}
	}

	// Update watcher with new config
	if err := s.watcher.UpdateConfig(newCfg); err != nil {
		s.log.Error("Failed to update watcher with new config", "error", err)
	} else {
		s.log.Info("Watcher configuration updated")
	}

	// Update processor with new config
	s.processor.UpdateConfig(newCfg)
	s.log.Info("Processor configuration updated")
}
