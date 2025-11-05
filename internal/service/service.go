package service

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gitrgoliveira/vault_file_encryption/internal/config"
	"github.com/gitrgoliveira/vault_file_encryption/internal/crypto"
	"github.com/gitrgoliveira/vault_file_encryption/internal/interfaces"
	"github.com/gitrgoliveira/vault_file_encryption/internal/logger"
	"github.com/gitrgoliveira/vault_file_encryption/internal/queue"
	"github.com/gitrgoliveira/vault_file_encryption/internal/vault"
	"github.com/gitrgoliveira/vault_file_encryption/internal/watcher"
)

// Service encapsulates the watch service lifecycle
type Service struct {
	cfgMgr      interfaces.ConfigManager
	log         interfaces.Logger
	vaultClient interfaces.VaultClient
	encryptor   *crypto.Encryptor
	decryptor   *crypto.Decryptor
	queue       interfaces.Queue
	watcher     interfaces.Watcher
	processor   interfaces.Processor
	cancel      context.CancelFunc
}

// Config holds service configuration
type Config struct {
	ConfigFile string
	SignalChan <-chan os.Signal
}

// New creates a new service instance
func New(cfg *Config) (*Service, error) {
	// Create configuration manager for hot-reload support
	cfgMgr, err := config.NewManager(cfg.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate initial configuration
	if err := cfgMgr.Get().Validate(); err != nil {
		return nil, fmt.Errorf("invalid initial configuration: %w", err)
	}

	svc := &Service{
		cfgMgr: cfgMgr,
	}

	appCfg := cfgMgr.Get()

	// Setup logging
	if err := svc.setupLogging(appCfg); err != nil {
		return nil, err
	}

	// Setup Vault client and crypto components
	if err := svc.setupVaultAndCrypto(appCfg); err != nil {
		return nil, err
	}

	// Setup queue
	if err := svc.setupQueue(appCfg); err != nil {
		return nil, err
	}

	// Setup watcher and processor
	if err := svc.setupWatcherAndProcessor(appCfg); err != nil {
		return nil, err
	}

	// Register config reload callback
	svc.registerReloadCallback()

	return svc, nil
}

// setupLogging initializes the logger and optional audit logger
func (s *Service) setupLogging(cfg *config.Config) error {
	var log logger.Logger
	var err error

	// Create logger with optional audit logging
	var opts []logger.LoggerOption
	if cfg.Logging.AuditLog {
		opts = append(opts, logger.WithAudit(cfg.Logging.AuditPath))
	}

	log, err = logger.New(cfg.Logging.Level, cfg.Logging.Output, opts...)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	s.log = log

	if cfg.Logging.AuditLog {
		s.log.Info("Audit logging enabled", "audit_path", cfg.Logging.AuditPath)
	}

	return nil
}

// setupVaultAndCrypto creates Vault client and crypto components
func (s *Service) setupVaultAndCrypto(cfg *config.Config) error {
	vaultClient, err := vault.NewClient(&vault.Config{
		AgentAddress: cfg.Vault.AgentAddress,
		TransitMount: cfg.Vault.TransitMount,
		KeyName:      cfg.Vault.KeyName,
		Timeout:      cfg.Vault.RequestTimeout.Duration(),
	})
	if err != nil {
		return fmt.Errorf("failed to create Vault client: %w", err)
	}

	s.vaultClient = vaultClient
	s.encryptor = crypto.NewEncryptor(vaultClient, &crypto.EncryptorConfig{
		ChunkSize: cfg.Encryption.ChunkSize,
	})
	s.decryptor = crypto.NewDecryptor(vaultClient, &crypto.EncryptorConfig{
		ChunkSize: cfg.Encryption.ChunkSize,
	})

	return nil
}

// setupQueue creates and loads the queue
func (s *Service) setupQueue(cfg *config.Config) error {
	q, err := queue.NewQueue(&queue.Config{
		MaxRetries: cfg.Queue.MaxRetries,
		BaseDelay:  cfg.Queue.BaseDelay.Duration(),
		MaxDelay:   cfg.Queue.MaxDelay.Duration(),
		StatePath:  cfg.Queue.StatePath,
	})
	if err != nil {
		return fmt.Errorf("failed to create queue: %w", err)
	}

	// Load queue state if it exists
	if err := q.Load(); err != nil {
		s.log.Error("Failed to load queue state", "error", err)
	} else {
		s.log.Info("Queue state loaded", "items", q.Size())
	}

	s.queue = q
	return nil
}

// setupWatcherAndProcessor creates watcher and processor components
func (s *Service) setupWatcherAndProcessor(cfg *config.Config) error {
	w, err := watcher.NewWatcher(&watcher.Config{
		EncryptSourceDir:  cfg.Encryption.SourceDir,
		EncryptDestDir:    cfg.Encryption.DestDir,
		DecryptSourceDir:  cfg.Decryption.SourceDir,
		DecryptDestDir:    cfg.Decryption.DestDir,
		StabilityDuration: cfg.Queue.StabilityDuration.Duration(),
	}, s.queue, s.log)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	processor, err := watcher.NewProcessor(&watcher.ProcessorConfig{
		SourceFileBehavior: cfg.Encryption.SourceFileBehavior,
		ArchiveDir:         cfg.ArchiveDir("encrypt"),
		FailedDir:          cfg.FailedDir("encrypt"),
		DLQDir:             cfg.DLQDir("encrypt"),
		CalculateChecksum:  cfg.Encryption.CalculateChecksum,
		VerifyChecksum:     cfg.Decryption.VerifyChecksum,
	}, s.queue, s.encryptor, s.decryptor, s.log)
	if err != nil {
		return fmt.Errorf("failed to create processor: %w", err)
	}

	s.watcher = w
	s.processor = processor

	return nil
}

// registerReloadCallback sets up the config reload handler
func (s *Service) registerReloadCallback() {
	s.cfgMgr.OnReload(func(newCfg *config.Config) {
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
	})
}

// Run starts the service and blocks until shutdown
func (s *Service) Run(ctx context.Context, sigChan <-chan os.Signal, isReloadSignal, isShutdownSignal func(os.Signal) bool) error {
	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	defer cancel()

	// Start watcher and processor
	go func() {
		if err := s.watcher.Start(ctx); err != nil {
			s.log.Error("Watcher stopped with error", "error", err)
		}
	}()

	go func() {
		if err := s.processor.Start(ctx); err != nil {
			s.log.Error("Processor stopped with error", "error", err)
		}
	}()

	s.log.Info("File watcher service started - waiting for signals")
	s.log.Info("Press Ctrl+C to stop, or send SIGHUP to reload configuration (Unix only)")

	// Handle signals
	for {
		select {
		case sig := <-sigChan:
			if isReloadSignal(sig) {
				s.log.Info("Received reload signal, reloading configuration")
				if err := s.cfgMgr.Reload(); err != nil {
					s.log.Error("Failed to reload configuration", "error", err)
					continue
				}
				// Callback will be triggered automatically
			} else if isShutdownSignal(sig) {
				s.log.Info("Received shutdown signal, gracefully shutting down")
				return s.Shutdown()
			} else {
				s.log.Info("Received unknown signal", "signal", sig)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// Shutdown performs graceful shutdown
func (s *Service) Shutdown() error {
	// Cancel context to stop all goroutines first
	if s.cancel != nil {
		s.cancel()
	}

	// Give goroutines time to finish current operations and stop queue modifications
	s.log.Info("Waiting for goroutines to finish")
	time.Sleep(100 * time.Millisecond)

	// Save queue state after all modifications have stopped
	s.log.Info("Saving queue state")
	if err := s.queue.Save(); err != nil {
		s.log.Error("Failed to save queue state", "error", err)
	}

	s.log.Info("Shutdown complete")
	return nil
}

// Close releases all resources
func (s *Service) Close() error {
	if s.log != nil {
		_ = s.log.Sync()
	}
	if s.vaultClient != nil {
		_ = s.vaultClient.Close()
	}
	return nil
}
