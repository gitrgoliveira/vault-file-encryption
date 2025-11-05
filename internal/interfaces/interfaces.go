package interfaces

import (
	"context"

	"github.com/gitrgoliveira/vault_file_encryption/internal/config"
	"github.com/gitrgoliveira/vault_file_encryption/internal/model"
)

// ConfigManager defines the interface for managing configuration.
type ConfigManager interface {
	Get() *config.Config
	Reload() error
	OnReload(func(*config.Config))
}

// Logger defines the interface for logging.
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
	Sync() error
}

// VaultClient defines the interface for interacting with Vault.
type VaultClient interface {
	Close() error
}

// Queue defines the interface for the processing queue.
type Queue interface {
	Load() error
	Save() error
	Size() int
	Enqueue(item *model.Item) error
	Dequeue() *model.Item
	Requeue(item *model.Item, err error) error
}

// Watcher defines the interface for the file watcher.
type Watcher interface {
	Start(ctx context.Context) error
	UpdateConfig(cfg *config.Config) error
}

// Processor defines the interface for the file processor.
type Processor interface {
	Start(ctx context.Context) error
	UpdateConfig(cfg *config.Config)
}
