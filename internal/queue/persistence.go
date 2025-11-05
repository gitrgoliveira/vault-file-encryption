package queue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gitrgoliveira/vault_file_encryption/internal/model"
)

// Persistence handles saving and loading queue state
type Persistence struct {
	statePath string
}

// NewPersistence creates a new persistence handler
func NewPersistence(statePath string) (*Persistence, error) {
	if statePath == "" {
		return nil, fmt.Errorf("state path cannot be empty")
	}

	// Ensure directory exists
	dir := filepath.Dir(statePath)
	if err := os.MkdirAll(dir, 0750); err != nil { // #nosec G301 - configurable directory path
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	return &Persistence{
		statePath: statePath,
	}, nil
}

// Save saves queue items to disk
func (p *Persistence) Save(items []*model.Item) error {
	// Create temporary file
	tmpPath := p.statePath + ".tmp"

	// Marshal items to JSON
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal queue state: %w", err)
	}

	// Write to temporary file
	if err := os.WriteFile(tmpPath, data, 0600); err != nil { // #nosec G306 - queue state file
		return fmt.Errorf("failed to write queue state: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, p.statePath); err != nil {
		return fmt.Errorf("failed to save queue state: %w", err)
	}

	return nil
}

// Load loads queue items from disk
func (p *Persistence) Load() ([]*model.Item, error) {
	// Check if file exists
	if _, err := os.Stat(p.statePath); os.IsNotExist(err) {
		// No state file, return empty list
		return []*model.Item{}, nil
	}

	// Read file
	data, err := os.ReadFile(p.statePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read queue state: %w", err)
	}

	// Unmarshal JSON
	var items []*model.Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("failed to unmarshal queue state: %w", err)
	}

	return items, nil
}
