package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
)

// RewrapDataKey re-wraps an encrypted DEK with the latest Vault Transit key version.
// This operation decrypts the ciphertext with the old key version and re-encrypts it
// with the latest key version, without exposing the plaintext DEK to the client.
//
// Input:  "vault:v1:ABC123..." (encrypted with key version 1)
// Output: "vault:v3:XYZ789..." (re-encrypted with latest key version 3)
//
// The file content itself is NOT re-encrypted - only the DEK in the .key file is updated.
func (c *Client) RewrapDataKey(ctx context.Context, ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", fmt.Errorf("ciphertext cannot be empty")
	}

	// Prepare request
	path := fmt.Sprintf("%s/rewrap/%s", c.config.TransitMount, c.config.KeyName)
	data := map[string]interface{}{
		"ciphertext": ciphertext,
	}

	// Make API call
	secret, err := c.client.Logical().WriteWithContext(ctx, path, data)
	if err != nil {
		return "", fmt.Errorf("vault rewrap failed: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return "", fmt.Errorf("vault returned empty response")
	}

	// Extract re-wrapped ciphertext
	newCiphertext, ok := secret.Data["ciphertext"].(string)
	if !ok || newCiphertext == "" {
		return "", fmt.Errorf("vault response missing ciphertext field")
	}

	return newCiphertext, nil
}

// GetKeyVersion extracts the key version number from Vault Transit ciphertext.
//
// Vault Transit ciphertext format: "vault:v{version}:{base64-ciphertext}"
// Examples:
//   - "vault:v1:ABC123..." returns 1
//   - "vault:v3:XYZ789..." returns 3
//
// Returns error if format is invalid or version cannot be parsed.
func GetKeyVersion(ciphertext string) (int, error) {
	// Pattern: vault:v{number}:{base64}
	// Example: vault:v1:ABC123...
	re := regexp.MustCompile(`^vault:v(\d+):`)
	matches := re.FindStringSubmatch(ciphertext)

	if len(matches) < 2 {
		return 0, fmt.Errorf("invalid vault ciphertext format: %s", ciphertext)
	}

	version, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid version number: %w", err)
	}

	return version, nil
}

// KeyVersionInfo contains information about a key file's version.
type KeyVersionInfo struct {
	FilePath    string // Path to the .key file
	Ciphertext  string // Current encrypted DEK
	Version     int    // Current key version
	NeedsRewrap bool   // True if version < minimum required
}

// GetKeyVersionInfo reads a .key file and extracts version information.
func GetKeyVersionInfo(filePath, ciphertext string, minVersion int) (*KeyVersionInfo, error) {
	version, err := GetKeyVersion(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to parse key version for %s: %w", filePath, err)
	}

	info := &KeyVersionInfo{
		FilePath:    filePath,
		Ciphertext:  ciphertext,
		Version:     version,
		NeedsRewrap: version < minVersion,
	}

	return info, nil
}

// RewrapResult contains the result of a rewrap operation.
type RewrapResult struct {
	FilePath      string // Path to the .key file
	OldVersion    int    // Version before rewrap
	NewVersion    int    // Version after rewrap
	OldCiphertext string // Ciphertext before rewrap
	NewCiphertext string // Ciphertext after rewrap
	BackupCreated bool   // Whether a backup was created
	Error         error  // Error if rewrap failed
}

// MarshalJSON implements custom JSON marshaling for audit logging.
func (r *RewrapResult) MarshalJSON() ([]byte, error) {
	// Create explicit map to avoid struct embedding issues
	return json.Marshal(map[string]interface{}{
		"FilePath":      r.FilePath,
		"OldVersion":    r.OldVersion,
		"NewVersion":    r.NewVersion,
		"OldCiphertext": r.OldCiphertext,
		"NewCiphertext": r.NewCiphertext,
		"BackupCreated": r.BackupCreated,
		"Error":         r.Error,
		"success":       r.Error == nil,
	})
}
