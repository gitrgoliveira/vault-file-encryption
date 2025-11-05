package config

import "time"

// Default timeout and duration constants
const (
	// DefaultVaultTimeout is the default timeout for Vault API requests
	DefaultVaultTimeout = 30 * time.Second

	// DefaultStabilityDuration is the default duration to wait for file stability
	DefaultStabilityDuration = 1 * time.Second

	// DefaultBaseDelay is the default initial retry delay
	DefaultBaseDelay = 1 * time.Second

	// DefaultMaxDelay is the default maximum retry delay
	DefaultMaxDelay = 5 * time.Minute

	// DefaultMaxRetries is the default maximum number of retry attempts
	DefaultMaxRetries = 3
)
