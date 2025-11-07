package watcher

import (
	"context"
	"fmt"
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
	queue           interfaces.Queue
	encryptStrategy ProcessStrategy
	decryptStrategy ProcessStrategy
	FileHandler     *FileHandler // Exposed for testing
	logger          logger.Logger
	mu              sync.RWMutex
}

// ProcessorConfig holds processor configuration
type ProcessorConfig struct {
	SourceFileBehavior string
	ArchiveDir         string
	FailedDir          string
	DLQDir             string
	CalculateChecksum  bool
	VerifyChecksum     bool
}

// NewProcessor creates a new file processor
func NewProcessor(
	cfg *ProcessorConfig,
	q interfaces.Queue,
	enc *crypto.Encryptor,
	dec *crypto.Decryptor,
	log logger.Logger,
) (*Processor, error) {
	// Create file handler
	fileHandler, err := NewFileHandler(&FileHandlerConfig{
		SourceFileBehavior: cfg.SourceFileBehavior,
		ArchiveDir:         cfg.ArchiveDir,
		FailedDir:          cfg.FailedDir,
		DLQDir:             cfg.DLQDir,
	}, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create file handler: %w", err)
	}

	// Create strategies
	encryptStrategy := NewEncryptStrategy(enc, log, cfg.CalculateChecksum)
	decryptStrategy := NewDecryptStrategy(dec, log, cfg.VerifyChecksum)

	return &Processor{
		queue:           q,
		encryptStrategy: encryptStrategy,
		decryptStrategy: decryptStrategy,
		FileHandler:     fileHandler,
		logger:          log,
	}, nil
}

// UpdateConfig safely updates the processor's configuration.
func (p *Processor) UpdateConfig(cfg *config.Config) {
	p.mu.Lock()
	defer p.mu.Unlock()

	newCfg := &ProcessorConfig{
		SourceFileBehavior: cfg.Encryption.SourceFileBehavior,
		ArchiveDir:         cfg.ArchiveDir("encrypt"),
		FailedDir:          cfg.FailedDir("encrypt"),
		DLQDir:             cfg.DLQDir("encrypt"),
		CalculateChecksum:  cfg.Encryption.CalculateChecksum,
		VerifyChecksum:     cfg.Decryption.VerifyChecksum,
	}

	// Update file handler
	p.FileHandler.UpdateConfig(&FileHandlerConfig{
		SourceFileBehavior: newCfg.SourceFileBehavior,
		ArchiveDir:         newCfg.ArchiveDir,
		FailedDir:          newCfg.FailedDir,
		DLQDir:             newCfg.DLQDir,
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

	switch item.Operation {
	case model.OperationEncrypt:
		strategy = p.encryptStrategy
	case model.OperationDecrypt:
		strategy = p.decryptStrategy
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
			p.FileHandler.MoveToDLQ(item)
		}

		// Move source file to failed directory
		p.FileHandler.MoveToFailed(item.SourcePath)

		return
	}

	// Mark as completed
	item.MarkCompleted()

	p.logger.Info("Successfully processed file",
		"id", item.ID,
		"file", item.SourcePath,
		"dest", item.DestPath,
	)

	// Handle source file
	p.FileHandler.HandleSourceFile(item.SourcePath)
}
