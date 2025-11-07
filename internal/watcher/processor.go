package watcher

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gitrgoliveira/vault-file-encryption/internal/config"
	"github.com/gitrgoliveira/vault-file-encryption/internal/crypto"
	"github.com/gitrgoliveira/vault-file-encryption/internal/interfaces"
	"github.com/gitrgoliveira/vault-file-encryption/internal/logger"
	"github.com/gitrgoliveira/vault-file-encryption/internal/model"
)

// Processor processes files from the queue
type Processor struct {
	queue              interfaces.Queue
	encryptStrategy    ProcessStrategy
	decryptStrategy    ProcessStrategy
	FileHandler        *FileHandler // Exposed for testing (encryption)
	decryptFileHandler *FileHandler // File handler for decryption
	logger             logger.Logger
	mu                 sync.RWMutex
}

// ProcessorConfig holds processor configuration
type ProcessorConfig struct {
	// Encryption configuration
	EncryptSourceFileBehavior string
	EncryptArchiveDir         string
	EncryptFailedDir          string
	EncryptDLQDir             string
	CalculateChecksum         bool

	// Decryption configuration
	DecryptSourceFileBehavior string
	DecryptArchiveDir         string
	DecryptFailedDir          string
	DecryptDLQDir             string
	VerifyChecksum            bool
}

// NewProcessor creates a new file processor
func NewProcessor(
	cfg *ProcessorConfig,
	q interfaces.Queue,
	enc *crypto.Encryptor,
	dec *crypto.Decryptor,
	log logger.Logger,
) (*Processor, error) {
	// Create encryption file handler
	encryptFileHandler, err := NewFileHandler(&FileHandlerConfig{
		SourceFileBehavior: cfg.EncryptSourceFileBehavior,
		ArchiveDir:         cfg.EncryptArchiveDir,
		FailedDir:          cfg.EncryptFailedDir,
		DLQDir:             cfg.EncryptDLQDir,
	}, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryption file handler: %w", err)
	}

	// Create decryption file handler
	decryptFileHandler, err := NewFileHandler(&FileHandlerConfig{
		SourceFileBehavior: cfg.DecryptSourceFileBehavior,
		ArchiveDir:         cfg.DecryptArchiveDir,
		FailedDir:          cfg.DecryptFailedDir,
		DLQDir:             cfg.DecryptDLQDir,
	}, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create decryption file handler: %w", err)
	}

	// Create strategies
	encryptStrategy := NewEncryptStrategy(enc, log, cfg.CalculateChecksum)
	decryptStrategy := NewDecryptStrategy(dec, log, cfg.VerifyChecksum)

	return &Processor{
		queue:              q,
		encryptStrategy:    encryptStrategy,
		decryptStrategy:    decryptStrategy,
		FileHandler:        encryptFileHandler, // For encryption (backward compatibility)
		decryptFileHandler: decryptFileHandler, // For decryption
		logger:             log,
	}, nil
}

// UpdateConfig safely updates the processor's configuration.
func (p *Processor) UpdateConfig(cfg *config.Config) {
	p.mu.Lock()
	defer p.mu.Unlock()

	newCfg := &ProcessorConfig{
		EncryptSourceFileBehavior: cfg.Encryption.SourceFileBehavior,
		EncryptArchiveDir:         cfg.ArchiveDir("encrypt"),
		EncryptFailedDir:          cfg.FailedDir("encrypt"),
		EncryptDLQDir:             cfg.DLQDir("encrypt"),
		CalculateChecksum:         cfg.Encryption.CalculateChecksum,
		DecryptSourceFileBehavior: cfg.Decryption.SourceFileBehavior,
		DecryptArchiveDir:         cfg.ArchiveDir("decrypt"),
		DecryptFailedDir:          cfg.FailedDir("decrypt"),
		DecryptDLQDir:             cfg.DLQDir("decrypt"),
		VerifyChecksum:            cfg.Decryption.VerifyChecksum,
	}

	// Update encryption file handler
	p.FileHandler.UpdateConfig(&FileHandlerConfig{
		SourceFileBehavior: newCfg.EncryptSourceFileBehavior,
		ArchiveDir:         newCfg.EncryptArchiveDir,
		FailedDir:          newCfg.EncryptFailedDir,
		DLQDir:             newCfg.EncryptDLQDir,
	})

	// Update decryption file handler
	p.decryptFileHandler.UpdateConfig(&FileHandlerConfig{
		SourceFileBehavior: newCfg.DecryptSourceFileBehavior,
		ArchiveDir:         newCfg.DecryptArchiveDir,
		FailedDir:          newCfg.DecryptFailedDir,
		DLQDir:             newCfg.DecryptDLQDir,
	})

	// Update strategies with new configuration
	// Safe type assertions: these strategies are always set to concrete types in NewProcessor
	if encryptStrategy, ok := p.encryptStrategy.(*EncryptStrategy); ok {
		p.encryptStrategy = &EncryptStrategy{
			encryptor:         encryptStrategy.encryptor,
			logger:            p.logger,
			calculateChecksum: newCfg.CalculateChecksum,
		}
	}

	if decryptStrategy, ok := p.decryptStrategy.(*DecryptStrategy); ok {
		p.decryptStrategy = &DecryptStrategy{
			decryptor:      decryptStrategy.decryptor,
			logger:         p.logger,
			verifyChecksum: newCfg.VerifyChecksum,
		}
	}
}

// Start starts processing files from the queue
func (p *Processor) Start(ctx context.Context) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			// Try to process next item
			item := p.queue.Dequeue()
			if item == nil {
				continue
			}

			p.processItem(ctx, item)
		}
	}
}

// processItem processes a single queue item
func (p *Processor) processItem(ctx context.Context, item *model.Item) {
	item.MarkProcessing()

	p.mu.RLock()
	defer p.mu.RUnlock()

	p.logger.Info("Processing file",
		"id", item.ID,
		"operation", item.Operation,
		"file", item.SourcePath,
		"attempt", item.AttemptCount,
	)

	var err error
	var strategy ProcessStrategy
	var fileHandler *FileHandler

	switch item.Operation {
	case model.OperationEncrypt:
		strategy = p.encryptStrategy
		fileHandler = p.FileHandler
	case model.OperationDecrypt:
		strategy = p.decryptStrategy
		fileHandler = p.decryptFileHandler
	default:
		err = fmt.Errorf("unknown operation: %s", item.Operation)
	}

	if err == nil && strategy != nil {
		err = strategy.Process(ctx, item)
	}

	if err != nil {
		p.logger.Error("Failed to process file",
			"id", item.ID,
			"file", item.SourcePath,
			"error", err,
		)

		// Requeue for retry
		if err := p.queue.Requeue(item, err); err != nil {
			p.logger.Error("Failed to requeue item", "id", item.ID, "error", err)

			// Move to dead letter queue
			if fileHandler != nil {
				fileHandler.MoveToDLQ(item)
			}
		}

		// Move source file to failed directory
		if fileHandler != nil {
			fileHandler.MoveToFailed(item.SourcePath)
		}

		return
	}

	// Mark as completed
	item.MarkCompleted()

	p.logger.Info("Successfully processed file",
		"id", item.ID,
		"file", item.SourcePath,
		"dest", item.DestPath,
	)

	// Handle source file with the appropriate file handler
	if fileHandler != nil {
		fileHandler.HandleSourceFile(item.SourcePath)

		// For decryption, also handle the key file and checksum file
		if item.Operation == model.OperationDecrypt {
			if item.KeyPath != "" {
				fileHandler.HandleSourceFile(item.KeyPath)
			}

			// Handle checksum file if it exists (based on original filename without .enc)
			// Example: file.txt.enc -> file.txt.sha256
			checksumPath := strings.TrimSuffix(item.SourcePath, ".enc") + ".sha256"
			if _, err := os.Stat(checksumPath); err == nil {
				fileHandler.HandleSourceFile(checksumPath)
			}
		}
	}
}
