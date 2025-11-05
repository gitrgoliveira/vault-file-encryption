package vault

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetKeyVersion(t *testing.T) {
	tests := []struct {
		name        string
		ciphertext  string
		expected    int
		expectError bool
	}{
		{
			name:       "version 1",
			ciphertext: "vault:v1:ABC123DEF456",
			expected:   1,
		},
		{
			name:       "version 3",
			ciphertext: "vault:v3:XYZ789",
			expected:   3,
		},
		{
			name:       "version 10",
			ciphertext: "vault:v10:LONGVERSION",
			expected:   10,
		},
		{
			name:        "invalid format - no prefix",
			ciphertext:  "v1:ABC123",
			expectError: true,
		},
		{
			name:        "invalid format - no version",
			ciphertext:  "vault:ABC123",
			expectError: true,
		},
		{
			name:        "invalid format - non-numeric version",
			ciphertext:  "vault:vabc:ABC123",
			expectError: true,
		},
		{
			name:        "empty ciphertext",
			ciphertext:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := GetKeyVersion(tt.ciphertext)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, version)
		})
	}
}

func TestGetKeyVersionInfo(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		ciphertext  string
		minVersion  int
		expectError bool
		needsRewrap bool
	}{
		{
			name:        "version 1, min 3 - needs rewrap",
			filePath:    "/data/file.txt.key",
			ciphertext:  "vault:v1:ABC123",
			minVersion:  3,
			needsRewrap: true,
		},
		{
			name:        "version 3, min 3 - no rewrap needed",
			filePath:    "/data/file.txt.key",
			ciphertext:  "vault:v3:XYZ789",
			minVersion:  3,
			needsRewrap: false,
		},
		{
			name:        "version 5, min 3 - no rewrap needed",
			filePath:    "/data/file.txt.key",
			ciphertext:  "vault:v5:LATEST",
			minVersion:  3,
			needsRewrap: false,
		},
		{
			name:        "invalid ciphertext",
			filePath:    "/data/file.txt.key",
			ciphertext:  "invalid",
			minVersion:  3,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := GetKeyVersionInfo(tt.filePath, tt.ciphertext, tt.minVersion)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.filePath, info.FilePath)
			assert.Equal(t, tt.ciphertext, info.Ciphertext)
			assert.Equal(t, tt.needsRewrap, info.NeedsRewrap)
		})
	}
}

func TestRewrapDataKey(t *testing.T) {
	tests := []struct {
		name           string
		inputCipher    string
		vaultResponse  map[string]interface{}
		expectedCipher string
		expectError    bool
	}{
		{
			name:        "successful rewrap v1 to v3",
			inputCipher: "vault:v1:ABC123",
			vaultResponse: map[string]interface{}{
				"data": map[string]interface{}{
					"ciphertext": "vault:v3:XYZ789",
				},
			},
			expectedCipher: "vault:v3:XYZ789",
		},
		{
			name:        "empty ciphertext input",
			inputCipher: "",
			expectError: true,
		},
		{
			name:        "vault returns empty response",
			inputCipher: "vault:v1:ABC123",
			vaultResponse: map[string]interface{}{
				"data": nil,
			},
			expectError: true,
		},
		{
			name:        "vault returns no ciphertext field",
			inputCipher: "vault:v1:ABC123",
			vaultResponse: map[string]interface{}{
				"data": map[string]interface{}{
					"other_field": "value",
				},
			},
			expectError: true,
		},
		{
			name:        "vault returns empty ciphertext",
			inputCipher: "vault:v1:ABC123",
			vaultResponse: map[string]interface{}{
				"data": map[string]interface{}{
					"ciphertext": "",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock Vault server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and path (Vault SDK uses PUT, not POST)
				assert.Equal(t, http.MethodPut, r.Method)
				assert.Contains(t, r.URL.Path, "/rewrap/")

				// Return response
				if tt.vaultResponse != nil {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(tt.vaultResponse)
				} else {
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
			defer server.Close()

			// Create client
			cfg := &Config{
				AgentAddress: server.URL,
				TransitMount: "transit",
				KeyName:      "test-key",
			}
			client, err := NewClient(cfg)
			require.NoError(t, err)

			// Test rewrap
			ctx := context.Background()
			newCipher, err := client.RewrapDataKey(ctx, tt.inputCipher)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedCipher, newCipher)
		})
	}
}

func TestRewrapDataKey_VaultError(t *testing.T) {
	// Create mock Vault server that returns 500 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []string{"internal server error"},
		})
	}))
	defer server.Close()

	cfg := &Config{
		AgentAddress: server.URL,
		TransitMount: "transit",
		KeyName:      "test-key",
	}
	client, err := NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.RewrapDataKey(ctx, "vault:v1:ABC123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault rewrap failed")
}

func TestRewrapResult_MarshalJSON(t *testing.T) {
	tests := []struct {
		name          string
		result        RewrapResult
		expectSuccess bool
	}{
		{
			name: "successful rewrap",
			result: RewrapResult{
				FilePath:      "/data/file.txt.key",
				OldVersion:    1,
				NewVersion:    3,
				OldCiphertext: "vault:v1:OLD",
				NewCiphertext: "vault:v3:NEW",
				BackupCreated: true,
				Error:         nil,
			},
			expectSuccess: true,
		},
		{
			name: "failed rewrap",
			result: RewrapResult{
				FilePath:   "/data/file.txt.key",
				OldVersion: 1,
				Error:      assert.AnError,
			},
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal pointer to ensure custom MarshalJSON is called
			data, err := json.Marshal(&tt.result)
			require.NoError(t, err)

			var result map[string]interface{}
			err = json.Unmarshal(data, &result)
			require.NoError(t, err)

			// Verify basic fields
			assert.Equal(t, tt.result.FilePath, result["FilePath"])
			assert.Equal(t, float64(tt.result.OldVersion), result["OldVersion"])
			assert.Equal(t, float64(tt.result.NewVersion), result["NewVersion"])

			// Verify success field
			success, ok := result["success"].(bool)
			require.True(t, ok, "success field should be a boolean")
			assert.Equal(t, tt.expectSuccess, success)
		})
	}
}
