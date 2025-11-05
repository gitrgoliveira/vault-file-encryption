package config

import (
	"fmt"
	"sync"
)

// Manager manages configuration with hot-reload support
type Manager struct {
	mu         sync.RWMutex
	config     *Config
	configPath string
	callbacks  []func(*Config)
}

// NewManager creates a new configuration manager
func NewManager(configPath string) (*Manager, error) {
	cfg, err := Load(configPath)
	if err != nil {
		return nil, err
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &Manager{
		config:     cfg,
		configPath: configPath,
		callbacks:  []func(*Config){},
	}, nil
}

// Get returns the current configuration (read-only)
func (m *Manager) Get() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.config
}

// Reload reloads the configuration from disk
func (m *Manager) Reload() error {
	newCfg, err := Load(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to reload configuration: %w", err)
	}

	// Validate new configuration
	if err := newCfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	m.mu.Lock()
	m.config = newCfg
	callbacks := m.callbacks
	m.mu.Unlock()

	// Notify callbacks
	for _, callback := range callbacks {
		callback(newCfg)
	}

	return nil
}

// OnReload registers a callback to be called when configuration is reloaded
func (m *Manager) OnReload(callback func(*Config)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callbacks = append(m.callbacks, callback)
}
