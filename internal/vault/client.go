package vault

import (
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/vault/api"
)

// Client wraps Vault API client configured to use Vault Agent
type Client struct {
	client *api.Client
	config *Config
}

// Config holds Vault client configuration
type Config struct {
	// Address of Vault Agent listener (not HCP Vault directly)
	AgentAddress string

	// Transit mount path
	TransitMount string

	// Transit key name
	KeyName string

	// Request timeout
	Timeout time.Duration
}

// NewClient creates a new Vault client that connects via Vault Agent
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Validate config
	if cfg.AgentAddress == "" {
		return nil, fmt.Errorf("agent address is required")
	}
	if cfg.TransitMount == "" {
		return nil, fmt.Errorf("transit mount path is required")
	}
	if cfg.KeyName == "" {
		return nil, fmt.Errorf("key name is required")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second // Default timeout
	}

	// Create Vault API config
	vaultConfig := api.DefaultConfig()
	vaultConfig.Address = cfg.AgentAddress

	// The Vault SDK's DefaultConfig() already provides a production-ready HTTP client with:
	// - Connection pooling (via cleanhttp.DefaultPooledClient)
	// - 60s timeout, 10s TLS handshake timeout
	// - HTTP/2 support, TLS 1.2+ minimum
	// - Built-in retry logic with exponential backoff
	// Override timeout if different from default
	if cfg.Timeout > 0 && cfg.Timeout != 60*time.Second {
		vaultConfig.Timeout = cfg.Timeout
	}

	// Create Vault client
	client, err := api.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault client: %w", err)
	}

	// In development, support direct Vault access with token and namespace from environment
	// In production, Vault Agent handles authentication automatically
	if token := os.Getenv("VAULT_TOKEN"); token != "" {
		client.SetToken(token)
	}
	if namespace := os.Getenv("VAULT_NAMESPACE"); namespace != "" {
		client.SetNamespace(namespace)
	}

	return &Client{
		client: client,
		config: cfg,
	}, nil
}

// Health checks if Vault Agent is accessible
func (c *Client) Health() error {
	return c.HealthWithRetry(3, 1*time.Second)
}

// HealthWithRetry checks if Vault Agent is accessible with retry logic
func (c *Client) HealthWithRetry(maxRetries int, retryDelay time.Duration) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
		}

		sys := c.client.Sys()
		health, err := sys.Health()
		if err != nil {
			lastErr = fmt.Errorf("vault health check failed (attempt %d/%d): %w", attempt+1, maxRetries+1, err)
			continue
		}

		if !health.Initialized {
			lastErr = fmt.Errorf("vault is not initialized (attempt %d/%d)", attempt+1, maxRetries+1)
			continue
		}

		if health.Sealed {
			lastErr = fmt.Errorf("vault is sealed (attempt %d/%d)", attempt+1, maxRetries+1)
			continue
		}

		// Success
		return nil
	}

	return lastErr
}

// Close performs cleanup.
func (c *Client) Close() error {
	// No-op for now, but can be used to close persistent connections
	// or clean up resources in the future.
	return nil
}
