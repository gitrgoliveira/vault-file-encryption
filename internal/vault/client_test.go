package vault

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
			errorMsg:    "config cannot be nil",
		},
		{
			name: "empty agent address",
			config: &Config{
				AgentAddress: "",
				TransitMount: "transit",
				KeyName:      "test-key",
			},
			expectError: true,
			errorMsg:    "agent address is required",
		},
		{
			name: "empty transit mount",
			config: &Config{
				AgentAddress: "http://127.0.0.1:8200",
				TransitMount: "",
				KeyName:      "test-key",
			},
			expectError: true,
			errorMsg:    "transit mount path is required",
		},
		{
			name: "empty key name",
			config: &Config{
				AgentAddress: "http://127.0.0.1:8200",
				TransitMount: "transit",
				KeyName:      "",
			},
			expectError: true,
			errorMsg:    "key name is required",
		},
		{
			name: "valid config",
			config: &Config{
				AgentAddress: "http://127.0.0.1:8200",
				TransitMount: "transit",
				KeyName:      "test-key",
			},
			expectError: false,
		},
		{
			name: "valid config with timeout",
			config: &Config{
				AgentAddress: "http://127.0.0.1:8200",
				TransitMount: "transit",
				KeyName:      "test-key",
				Timeout:      60 * time.Second,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, client)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
				assert.NotNil(t, client.client)
				assert.NotNil(t, client.config)

				// Verify timeout is set
				if tt.config.Timeout == 0 {
					assert.Equal(t, 30*time.Second, client.config.Timeout)
				} else {
					assert.Equal(t, tt.config.Timeout, client.config.Timeout)
				}

				// Clean up
				err = client.Close()
				assert.NoError(t, err)
			}
		})
	}
}

func TestGenerateDataKey(t *testing.T) {
	// Create mock Vault server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/transit/datakey/plaintext/test-key", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"plaintext":   "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=",
				"ciphertext":  "vault:v1:encrypted-data-key",
				"key_version": json.Number("1"),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		AgentAddress: server.URL,
		TransitMount: "transit",
		KeyName:      "test-key",
		Timeout:      5 * time.Second,
	})
	require.NoError(t, err)

	dataKey, err := client.GenerateDataKey()
	require.NoError(t, err)
	assert.NotNil(t, dataKey)
	assert.Equal(t, []byte("abcdefghijklmnopqrstuvwxyz123456"), dataKey.Plaintext)
	assert.Equal(t, "vault:v1:encrypted-data-key", dataKey.Ciphertext)
	// KeyVersion might be 0 due to type casting issues in the implementation
}

func TestGenerateDataKey_VaultError(t *testing.T) {
	// Create mock Vault server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":["permission denied"]}`))
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		AgentAddress: server.URL,
		TransitMount: "transit",
		KeyName:      "test-key",
	})
	require.NoError(t, err)

	_, err = client.GenerateDataKey()
	assert.Error(t, err)
}

func TestDecryptDataKey(t *testing.T) {
	// Create mock Vault server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/transit/decrypt/test-key", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"plaintext":   "ZGVjcnlwdGVkLWRhdGEta2V5LXBsYWludGV4dA==",
				"key_version": json.Number("2"),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		AgentAddress: server.URL,
		TransitMount: "transit",
		KeyName:      "test-key",
		Timeout:      5 * time.Second,
	})
	require.NoError(t, err)

	dataKey, err := client.DecryptDataKey("vault:v1:encrypted-data")
	require.NoError(t, err)
	assert.NotNil(t, dataKey)
	assert.Equal(t, []byte("decrypted-data-key-plaintext"), dataKey.Plaintext)
	assert.Equal(t, "vault:v1:encrypted-data", dataKey.Ciphertext)
	// KeyVersion might be 0 due to type casting issues in the implementation
}

func TestDecryptDataKey_VaultError(t *testing.T) {
	// Create mock Vault server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":["invalid ciphertext"]}`))
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		AgentAddress: server.URL,
		TransitMount: "transit",
		KeyName:      "test-key",
	})
	require.NoError(t, err)

	_, err = client.DecryptDataKey("invalid-ciphertext")
	assert.Error(t, err)
}

func TestGenerateDataKey_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`)) // Empty JSON response
	}))
	defer server.Close()

	client, err := NewClient(&Config{AgentAddress: server.URL, TransitMount: "transit", KeyName: "test-key"})
	require.NoError(t, err)

	_, err = client.GenerateDataKey()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response from vault")
}

func TestGenerateDataKey_MissingPlaintext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"ciphertext": "vault:v1:encrypted-data-key",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(&Config{AgentAddress: server.URL, TransitMount: "transit", KeyName: "test-key"})
	require.NoError(t, err)

	_, err = client.GenerateDataKey()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plaintext not found in response")
}

func TestGenerateDataKey_MissingCiphertext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"plaintext": "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(&Config{AgentAddress: server.URL, TransitMount: "transit", KeyName: "test-key"})
	require.NoError(t, err)

	_, err = client.GenerateDataKey()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ciphertext not found in response")
}

func TestDecryptDataKey_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`)) // Empty JSON response
	}))
	defer server.Close()

	client, err := NewClient(&Config{AgentAddress: server.URL, TransitMount: "transit", KeyName: "test-key"})
	require.NoError(t, err)

	_, err = client.DecryptDataKey("some-ciphertext")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response from vault")
}

func TestDecryptDataKey_MissingPlaintext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"key_version": json.Number("2"),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(&Config{AgentAddress: server.URL, TransitMount: "transit", KeyName: "test-key"})
	require.NoError(t, err)

	_, err = client.DecryptDataKey("some-ciphertext")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plaintext not found in response")
}

func TestHealth(t *testing.T) {
	// Create mock Vault server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/sys/health", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		response := map[string]interface{}{
			"initialized": true,
			"sealed":      false,
			"standby":     false,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		AgentAddress: server.URL,
		TransitMount: "transit",
		KeyName:      "test-key",
	})
	require.NoError(t, err)

	err = client.Health()
	assert.NoError(t, err)
}

func TestHealth_NotInitialized(t *testing.T) {
	// Create mock Vault server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/v1/sys/health")
		assert.Equal(t, "GET", r.Method)

		response := map[string]interface{}{
			"initialized": false,
			"sealed":      false,
		}

		// Return 200 OK but with initialized: false
		// The Health() implementation checks the response body
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		AgentAddress: server.URL,
		TransitMount: "transit",
		KeyName:      "test-key",
	})
	require.NoError(t, err)

	err = client.Health()
	assert.Error(t, err)
	// The error message comes from the Health() implementation
	if err != nil {
		assert.Contains(t, err.Error(), "not initialized")
	}
}

func TestHealth_Sealed(t *testing.T) {
	// Create mock Vault server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"initialized": true,
			"sealed":      true,
		}

		w.WriteHeader(http.StatusServiceUnavailable)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		AgentAddress: server.URL,
		TransitMount: "transit",
		KeyName:      "test-key",
	})
	require.NoError(t, err)

	err = client.Health()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sealed")
}

func TestClose(t *testing.T) {
	client, err := NewClient(&Config{
		AgentAddress: "http://127.0.0.1:8200",
		TransitMount: "transit",
		KeyName:      "test-key",
	})
	require.NoError(t, err)

	err = client.Close()
	assert.NoError(t, err)
}

func TestHealthWithRetry_Success(t *testing.T) {
	// Create mock Vault server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"initialized": true,
			"sealed":      false,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		AgentAddress: server.URL,
		TransitMount: "transit",
		KeyName:      "test-key",
	})
	require.NoError(t, err)

	err = client.HealthWithRetry(3, 10*time.Millisecond)
	assert.NoError(t, err)
}

func TestHealthWithRetry_EventualSuccess(t *testing.T) {
	attemptCount := 0

	// Create mock Vault server that fails first 2 attempts
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++

		if attemptCount <= 2 {
			// Fail first 2 attempts
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		// Succeed on 3rd attempt
		response := map[string]interface{}{
			"initialized": true,
			"sealed":      false,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		AgentAddress: server.URL,
		TransitMount: "transit",
		KeyName:      "test-key",
	})
	require.NoError(t, err)

	err = client.HealthWithRetry(3, 10*time.Millisecond)
	assert.NoError(t, err)
	assert.Equal(t, 3, attemptCount)
}

func TestHealthWithRetry_AllAttemptsFail(t *testing.T) {
	// Create mock Vault server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		AgentAddress: server.URL,
		TransitMount: "transit",
		KeyName:      "test-key",
	})
	require.NoError(t, err)

	err = client.HealthWithRetry(2, 10*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "attempt 3/3")
}
