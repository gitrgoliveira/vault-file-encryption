package vault

import (
	"encoding/base64"
	"fmt"

	"github.com/gitrgoliveira/go-fileencrypt/secure"
)

// DataKey represents a Vault Transit data key
type DataKey struct {
	// Plaintext is the plaintext data key
	// It is stored as bytes to allow secure zeroing
	Plaintext []byte

	// Ciphertext is the encrypted data key
	Ciphertext string

	// KeyVersion is the version of the transit key used
	KeyVersion int
}

// Destroy securely zeros the plaintext key from memory
func (dk *DataKey) Destroy() {
	if dk.Plaintext != nil {
		secure.Zero(dk.Plaintext)
		dk.Plaintext = nil
	}
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
	plaintextBase64, ok := secret.Data["plaintext"].(string)
	if !ok {
		return nil, fmt.Errorf("plaintext not found in response")
	}

	ciphertext, ok := secret.Data["ciphertext"].(string)
	if !ok {
		return nil, fmt.Errorf("ciphertext not found in response")
	}

	keyVersion, _ := secret.Data["key_version"].(int)

	// Decode plaintext immediately
	plaintext, err := base64.StdEncoding.DecodeString(plaintextBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode plaintext key: %w", err)
	}

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
	plaintextBase64, ok := secret.Data["plaintext"].(string)
	if !ok {
		return nil, fmt.Errorf("plaintext not found in response")
	}

	keyVersion, _ := secret.Data["key_version"].(int)

	// Decode plaintext immediately
	plaintext, err := base64.StdEncoding.DecodeString(plaintextBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode plaintext key: %w", err)
	}

	return &DataKey{
		Plaintext:  plaintext,
		Ciphertext: ciphertext,
		KeyVersion: keyVersion,
	}, nil
}
