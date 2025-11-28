package config

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

// Load loads configuration from an HCL file
func Load(path string) (*Config, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file not found: %s", path)
	}

	// Parse HCL file
	var cfg Config
	if err := hclsimple.DecodeFile(path, nil, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	// Set defaults and parse durations
	if err := cfg.SetDefaults(); err != nil {
		return nil, fmt.Errorf("failed to set defaults: %w", err)
	}

	return &cfg, nil
}

// LoadFromString loads configuration from an HCL string
func LoadFromString(filename, content string) (*Config, error) {
	var cfg Config
	if err := hclsimple.Decode(filename, []byte(content), nil, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	// Set defaults and parse durations
	if err := cfg.SetDefaults(); err != nil {
		return nil, fmt.Errorf("failed to set defaults: %w", err)
	}

	return &cfg, nil
}
