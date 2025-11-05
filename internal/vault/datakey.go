package vault

import (
	"encoding/base64"
	"fmt"
)

// DataKey represents a Vault Transit data key
type DataKey struct {
	// Plaintext is the plaintext data key (base64 encoded)
	Plaintext string

	// Ciphertext is the encrypted data key
	Ciphertext string

	// KeyVersion is the version of the transit key used
	KeyVersion int
}

// PlaintextBytes decodes and returns the plaintext key as bytes
func (dk *DataKey) PlaintextBytes() ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(dk.Plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode plaintext key: %w", err)
	}
	return data, nil
}

// GenerateDataKey generates a new data encryption key from Vault Transit
func (c *Client) GenerateDataKey() (*DataKey, error) {
	path := fmt.Sprintf("%s/datakey/plaintext/%s", c.config.TransitMount, c.config.KeyName)

	// Request a data key from Vault
	secret, err := c.client.Logical().Write(path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate data key: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("empty response from vault")
	}

	// Extract plaintext and ciphertext
	plaintext, ok := secret.Data["plaintext"].(string)
	if !ok {
		return nil, fmt.Errorf("plaintext not found in response")
	}

	ciphertext, ok := secret.Data["ciphertext"].(string)
	if !ok {
		return nil, fmt.Errorf("ciphertext not found in response")
	}

	keyVersion, _ := secret.Data["key_version"].(int)

	return &DataKey{
		Plaintext:  plaintext,
		Ciphertext: ciphertext,
		KeyVersion: keyVersion,
	}, nil
}

// DecryptDataKey decrypts an encrypted data key using Vault Transit
func (c *Client) DecryptDataKey(ciphertext string) (*DataKey, error) {
	path := fmt.Sprintf("%s/decrypt/%s", c.config.TransitMount, c.config.KeyName)

	// Prepare request data
	data := map[string]interface{}{
		"ciphertext": ciphertext,
	}

	// Request decryption from Vault
	secret, err := c.client.Logical().Write(path, data)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data key: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("empty response from vault")
	}

	// Extract plaintext
	plaintext, ok := secret.Data["plaintext"].(string)
	if !ok {
		return nil, fmt.Errorf("plaintext not found in response")
	}

	keyVersion, _ := secret.Data["key_version"].(int)

	return &DataKey{
		Plaintext:  plaintext,
		Ciphertext: ciphertext,
		KeyVersion: keyVersion,
	}, nil
}
