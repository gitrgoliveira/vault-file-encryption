package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gitrgoliveira/vault-file-encryption/internal/config"
	"github.com/hashicorp/vault/api"
)

// Client wraps Vault API client configured to use Vault Agent
type Client struct {
	client *api.Client
	config *Config
}

// Config holds Vault client configuration
type Config struct {
	// Vault Agent listener address
	AgentAddress string

	// Transit mount path
	TransitMount string

	// Transit key name
	KeyName string

	// Request timeout
	Timeout time.Duration

	// Auth configuration
	Auth *config.AuthConfig
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

	// Vault SDK provides production-ready defaults (pooling, retry, TLS 1.2+, 60s timeout)
	// Override timeout if different from default
	if cfg.Timeout > 0 && cfg.Timeout != 60*time.Second {
		vaultConfig.Timeout = cfg.Timeout
	}

	// Create Vault client
	apiClient, err := api.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault client: %w", err)
	}

	client := &Client{
		client: apiClient,
		config: cfg,
	}

	// Authenticate if auth config is present
	if cfg.Auth != nil {
		if err := client.login(cfg.Auth); err != nil {
			return nil, fmt.Errorf("failed to authenticate: %w", err)
		}
	} else {
		// In development, support direct Vault access with token and namespace from environment
		// In production, Vault Agent handles authentication automatically
		if token := os.Getenv("VAULT_TOKEN"); token != "" {
			apiClient.SetToken(token)
		}
		if namespace := os.Getenv("VAULT_NAMESPACE"); namespace != "" {
			apiClient.SetNamespace(namespace)
		}
	}

	return client, nil
}

// login handles authentication based on configuration
func (c *Client) login(authCfg *config.AuthConfig) error {
	switch authCfg.Method {
	case "token":
		return c.authToken(authCfg.Token)
	case "approle":
		return c.authAppRole(authCfg.AppRole)
	case "kubernetes":
		return c.authKubernetes(authCfg.Kubernetes)
	case "jwt":
		return c.authJWT(authCfg.JWT)
	case "cert":
		return c.authCert(authCfg.Cert)
	case "agent":
		// Do nothing, rely on Agent
		return nil
	default:
		return fmt.Errorf("unknown auth method: %s", authCfg.Method)
	}
}

func (c *Client) authToken(cfg *config.TokenAuthConfig) error {
	token := cfg.Token
	if token == "" {
		token = os.Getenv("VAULT_TOKEN")
	}
	if token == "" {
		return fmt.Errorf("token is required")
	}
	c.client.SetToken(token)
	return nil
}

func (c *Client) authAppRole(cfg *config.AppRoleAuthConfig) error {
	roleID := cfg.RoleID
	if roleID == "" {
		roleID = os.Getenv("VAULT_ROLE_ID")
	}
	if roleID == "" {
		return fmt.Errorf("role_id is required")
	}

	secretID := cfg.SecretID
	if secretID == "" {
		secretID = os.Getenv("VAULT_SECRET_ID")
	}
	if secretID == "" {
		return fmt.Errorf("secret_id is required")
	}

	mountPath := cfg.MountPath
	if mountPath == "" {
		mountPath = "auth/approle"
	}

	data := map[string]interface{}{
		"role_id":   roleID,
		"secret_id": secretID,
	}

	path := fmt.Sprintf("%s/login", mountPath)
	secret, err := c.client.Logical().Write(path, data)
	if err != nil {
		return fmt.Errorf("failed to login with approle: %w", err)
	}
	if secret == nil || secret.Auth == nil {
		return fmt.Errorf("login returned no auth info")
	}

	c.client.SetToken(secret.Auth.ClientToken)
	return nil
}

func (c *Client) authKubernetes(cfg *config.KubernetesAuthConfig) error {
	role := cfg.Role
	if role == "" {
		return fmt.Errorf("role is required")
	}

	tokenPath := cfg.TokenPath
	if tokenPath == "" {
		tokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	}

	// Read the service account token
	// #nosec G304 - tokenPath is from configuration, not user input
	jwt, err := os.ReadFile(filepath.Clean(tokenPath))
	if err != nil {
		return fmt.Errorf("failed to read kubernetes service account token: %w", err)
	}

	mountPath := cfg.MountPath
	if mountPath == "" {
		mountPath = "auth/kubernetes"
	}

	data := map[string]interface{}{
		"role": role,
		"jwt":  string(jwt),
	}

	path := fmt.Sprintf("%s/login", mountPath)
	secret, err := c.client.Logical().Write(path, data)
	if err != nil {
		return fmt.Errorf("failed to login with kubernetes: %w", err)
	}
	if secret == nil || secret.Auth == nil {
		return fmt.Errorf("login returned no auth info")
	}

	c.client.SetToken(secret.Auth.ClientToken)
	return nil
}

func (c *Client) authJWT(cfg *config.JWTAuthConfig) error {
	role := cfg.Role
	if role == "" {
		return fmt.Errorf("role is required")
	}

	path := cfg.Path
	if path == "" {
		return fmt.Errorf("path to jwt is required")
	}

	// Read the JWT
	// #nosec G304 - path is from configuration, not user input
	jwt, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("failed to read jwt file: %w", err)
	}

	mountPath := cfg.MountPath
	if mountPath == "" {
		mountPath = "auth/jwt"
	}

	data := map[string]interface{}{
		"role": role,
		"jwt":  string(jwt),
	}

	loginPath := fmt.Sprintf("%s/login", mountPath)
	secret, err := c.client.Logical().Write(loginPath, data)
	if err != nil {
		return fmt.Errorf("failed to login with jwt: %w", err)
	}
	if secret == nil || secret.Auth == nil {
		return fmt.Errorf("login returned no auth info")
	}

	c.client.SetToken(secret.Auth.ClientToken)
	return nil
}

func (c *Client) authCert(cfg *config.CertAuthConfig) error {
	// Cert auth requires TLS configuration which should be done during client creation.
	// However, the login call is still needed to get a token.
	// The client certificate is presented during the TLS handshake.

	mountPath := cfg.MountPath
	if mountPath == "" {
		mountPath = "auth/cert"
	}

	// Name is optional; if not provided, Vault uses the cert's common name
	data := map[string]interface{}{
		"name": cfg.Name,
	}

	path := fmt.Sprintf("%s/login", mountPath)
	secret, err := c.client.Logical().Write(path, data)
	if err != nil {
		return fmt.Errorf("failed to login with cert: %w", err)
	}
	if secret == nil || secret.Auth == nil {
		return fmt.Errorf("login returned no auth info")
	}

	c.client.SetToken(secret.Auth.ClientToken)
	return nil
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
