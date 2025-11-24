package crypto

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gitrgoliveira/vault-file-encryption/internal/vault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockVaultClient is a simple mock for testing
type mockVaultClient struct {
	generateKeyFunc func() (*vault.DataKey, error)
	decryptKeyFunc  func(string) (*vault.DataKey, error)
}

func (m *mockVaultClient) GenerateDataKey() (*vault.DataKey, error) {
	if m.generateKeyFunc != nil {
		return m.generateKeyFunc()
	}
	// Return a valid test key (256 bits = 32 bytes)
	// Base64 "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" decodes to 32 bytes of zeros
	key := make([]byte, 32)
	return &vault.DataKey{
		Plaintext:  key,
		Ciphertext: "vault:v1:test-encrypted-key",
		KeyVersion: 1,
	}, nil
}

func (m *mockVaultClient) DecryptDataKey(ciphertext string) (*vault.DataKey, error) {
	if m.decryptKeyFunc != nil {
		return m.decryptKeyFunc(ciphertext)
	}
	// Base64 "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" decodes to 32 bytes of zeros
	key := make([]byte, 32)
	return &vault.DataKey{
		Plaintext:  key,
		Ciphertext: ciphertext,
		KeyVersion: 1,
	}, nil
}

func (m *mockVaultClient) Health() error {
	return nil
}

func (m *mockVaultClient) Close() error {
	return nil
}

func TestEncryptor_EncryptFile_SmallFile(t *testing.T) {
	// Test file smaller than chunk size
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "small.txt")
	destFile := filepath.Join(tmpDir, "small.enc")

	testContent := []byte("Small test content")
	require.NoError(t, os.WriteFile(sourceFile, testContent, 0644))

	mock := &mockVaultClient{}
	encryptor := NewEncryptor(mock, nil)

	ctx := context.Background()
	encryptedKey, err := encryptor.EncryptFile(ctx, sourceFile, destFile, nil)

	require.NoError(t, err)
	assert.NotEmpty(t, encryptedKey)
	assert.Equal(t, "vault:v1:test-encrypted-key", encryptedKey)

	// Verify encrypted file exists and is different
	encryptedData, err := os.ReadFile(destFile)
	require.NoError(t, err)
	assert.NotEqual(t, testContent, encryptedData)
	assert.Greater(t, len(encryptedData), len(testContent))
}

func TestEncryptor_EncryptFile_LargeFile(t *testing.T) {
	// Test file larger than chunk size (multiple chunks)
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "large.bin")
	destFile := filepath.Join(tmpDir, "large.enc")

	// Create 2.5MB file (more than 2 chunks)
	largeContent := make([]byte, DefaultChunkSize*2+DefaultChunkSize/2)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}
	require.NoError(t, os.WriteFile(sourceFile, largeContent, 0644))

	mock := &mockVaultClient{}
	encryptor := NewEncryptor(mock, nil)

	progressCalls := 0
	progressCallback := func(progress float64) {
		progressCalls++
		assert.GreaterOrEqual(t, progress, 0.0)
		assert.LessOrEqual(t, progress, 100.0)
	}

	ctx := context.Background()
	encryptedKey, err := encryptor.EncryptFile(ctx, sourceFile, destFile, progressCallback)

	require.NoError(t, err)
	assert.NotEmpty(t, encryptedKey)
	assert.Greater(t, progressCalls, 0, "Progress callback should be called")

	// Verify encrypted file exists
	encryptedData, err := os.ReadFile(destFile)
	require.NoError(t, err)
	assert.Greater(t, len(encryptedData), len(largeContent))
}

func TestEncryptor_EncryptFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "empty.txt")
	destFile := filepath.Join(tmpDir, "empty.enc")

	// Create empty file
	require.NoError(t, os.WriteFile(sourceFile, []byte{}, 0644))

	mock := &mockVaultClient{}
	encryptor := NewEncryptor(mock, nil)

	ctx := context.Background()
	encryptedKey, err := encryptor.EncryptFile(ctx, sourceFile, destFile, nil)

	require.NoError(t, err)
	assert.NotEmpty(t, encryptedKey)

	// Encrypted file should have content (header + salt + etc)
	encryptedData, err := os.ReadFile(destFile)
	require.NoError(t, err)
	assert.Greater(t, len(encryptedData), 0)
}

func TestEncryptor_EncryptFile_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "large.bin")
	destFile := filepath.Join(tmpDir, "large.enc")

	// Create large file
	largeContent := make([]byte, DefaultChunkSize*10)
	require.NoError(t, os.WriteFile(sourceFile, largeContent, 0644))

	mock := &mockVaultClient{}
	encryptor := NewEncryptor(mock, nil)

	// Create context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := encryptor.EncryptFile(ctx, sourceFile, destFile, nil)

	// Should get context canceled error
	assert.Error(t, err)
	// go-fileencrypt might wrap the error, check string if ErrorIs fails or just check string
	assert.Contains(t, err.Error(), "context canceled")
}

func TestEncryptor_EncryptFile_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "nonexistent.txt")
	destFile := filepath.Join(tmpDir, "output.enc")

	mock := &mockVaultClient{}
	encryptor := NewEncryptor(mock, nil)

	ctx := context.Background()
	_, err := encryptor.EncryptFile(ctx, sourceFile, destFile, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to encrypt file")
}

func TestDecryptor_DecryptFile_SmallFile(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	encryptedFile := filepath.Join(tmpDir, "encrypted.enc")
	keyFile := filepath.Join(tmpDir, "key.key")
	decryptedFile := filepath.Join(tmpDir, "decrypted.txt")

	testContent := []byte("Test content for decryption")
	require.NoError(t, os.WriteFile(sourceFile, testContent, 0644))

	mock := &mockVaultClient{}
	encryptor := NewEncryptor(mock, nil)
	decryptor := NewDecryptor(mock, nil)

	ctx := context.Background()

	// Encrypt
	encryptedKey, err := encryptor.EncryptFile(ctx, sourceFile, encryptedFile, nil)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyFile, []byte(encryptedKey), 0644))

	// Decrypt
	err = decryptor.DecryptFile(ctx, encryptedFile, keyFile, decryptedFile, nil)
	require.NoError(t, err)

	// Verify content matches
	decryptedContent, err := os.ReadFile(decryptedFile)
	require.NoError(t, err)
	assert.Equal(t, testContent, decryptedContent)
}

func TestDecryptor_DecryptFile_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.bin")
	encryptedFile := filepath.Join(tmpDir, "encrypted.enc")
	keyFile := filepath.Join(tmpDir, "key.key")
	decryptedFile := filepath.Join(tmpDir, "decrypted.bin")

	// Create 2.5MB file
	largeContent := make([]byte, DefaultChunkSize*2+DefaultChunkSize/2)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}
	require.NoError(t, os.WriteFile(sourceFile, largeContent, 0644))

	mock := &mockVaultClient{}
	encryptor := NewEncryptor(mock, nil)
	decryptor := NewDecryptor(mock, nil)

	ctx := context.Background()

	// Encrypt
	encryptedKey, err := encryptor.EncryptFile(ctx, sourceFile, encryptedFile, nil)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyFile, []byte(encryptedKey), 0644))

	// Decrypt with progress callback
	progressCalls := 0
	progressCallback := func(progress float64) {
		progressCalls++
	}

	err = decryptor.DecryptFile(ctx, encryptedFile, keyFile, decryptedFile, progressCallback)
	require.NoError(t, err)
	assert.Greater(t, progressCalls, 0)

	// Verify content matches
	decryptedContent, err := os.ReadFile(decryptedFile)
	require.NoError(t, err)
	assert.Equal(t, largeContent, decryptedContent)
}

func TestDecryptor_DecryptFile_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.bin")
	encryptedFile := filepath.Join(tmpDir, "encrypted.enc")
	keyFile := filepath.Join(tmpDir, "key.key")
	decryptedFile := filepath.Join(tmpDir, "decrypted.bin")

	// Create large file
	largeContent := make([]byte, DefaultChunkSize*10)
	require.NoError(t, os.WriteFile(sourceFile, largeContent, 0644))

	mock := &mockVaultClient{}
	encryptor := NewEncryptor(mock, nil)
	decryptor := NewDecryptor(mock, nil)

	ctx := context.Background()

	// Encrypt first
	encryptedKey, err := encryptor.EncryptFile(ctx, sourceFile, encryptedFile, nil)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyFile, []byte(encryptedKey), 0644))

	// Try to decrypt with canceled context
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	err = decryptor.DecryptFile(canceledCtx, encryptedFile, keyFile, decryptedFile, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestDecryptor_DecryptFile_InvalidKeyFile(t *testing.T) {
	tmpDir := t.TempDir()
	encryptedFile := filepath.Join(tmpDir, "encrypted.enc")
	keyFile := filepath.Join(tmpDir, "nonexistent.key")
	decryptedFile := filepath.Join(tmpDir, "decrypted.txt")

	// Create dummy encrypted file
	require.NoError(t, os.WriteFile(encryptedFile, []byte("dummy"), 0644))

	mock := &mockVaultClient{}
	decryptor := NewDecryptor(mock, nil)

	ctx := context.Background()
	err := decryptor.DecryptFile(ctx, encryptedFile, keyFile, decryptedFile, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read key file")
}

func TestDecryptor_DecryptFile_TamperedCiphertext(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	encryptedFile := filepath.Join(tmpDir, "encrypted.enc")
	keyFile := filepath.Join(tmpDir, "key.key")
	decryptedFile := filepath.Join(tmpDir, "decrypted.txt")

	testContent := []byte("Test content that will be tampered")
	require.NoError(t, os.WriteFile(sourceFile, testContent, 0644))

	mock := &mockVaultClient{}
	encryptor := NewEncryptor(mock, nil)
	decryptor := NewDecryptor(mock, nil)

	ctx := context.Background()

	// Encrypt
	encryptedKey, err := encryptor.EncryptFile(ctx, sourceFile, encryptedFile, nil)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyFile, []byte(encryptedKey), 0644))

	// Tamper with the encrypted file
	encryptedData, err := os.ReadFile(encryptedFile)
	require.NoError(t, err)
	// Flip a bit somewhere in the ciphertext (middle of file)
	tamperIndex := len(encryptedData) / 2
	if tamperIndex < len(encryptedData) {
		encryptedData[tamperIndex] = ^encryptedData[tamperIndex]
	}
	require.NoError(t, os.WriteFile(encryptedFile, encryptedData, 0644))

	// Attempt to decrypt
	err = decryptor.DecryptFile(ctx, encryptedFile, keyFile, decryptedFile, nil)

	// Should fail with a decryption error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")
}

func TestDecryptor_DecryptFile_InvalidDEK(t *testing.T) {
	tmpDir := t.TempDir()
	encryptedFile := filepath.Join(tmpDir, "encrypted.enc")
	keyFile := filepath.Join(tmpDir, "key.key")
	decryptedFile := filepath.Join(tmpDir, "decrypted.txt")

	// Create dummy files
	require.NoError(t, os.WriteFile(encryptedFile, []byte("dummy-encrypted-data"), 0644))
	require.NoError(t, os.WriteFile(keyFile, []byte("invalid-dek-format"), 0644))

	// Mock vault client to return an error on decryption
	mock := &mockVaultClient{
		decryptKeyFunc: func(s string) (*vault.DataKey, error) {
			return nil, assert.AnError
		},
	}
	decryptor := NewDecryptor(mock, nil)

	ctx := context.Background()
	err := decryptor.DecryptFile(ctx, encryptedFile, keyFile, decryptedFile, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decrypt data key")
	assert.ErrorIs(t, err, assert.AnError)
}

func TestDecryptor_DecryptFile_MismatchedKey(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// File A
	sourceA := filepath.Join(tmpDir, "sourceA.txt")
	encryptedA := filepath.Join(tmpDir, "encryptedA.enc")
	keyA := filepath.Join(tmpDir, "keyA.key")
	require.NoError(t, os.WriteFile(sourceA, []byte("content of file A"), 0644))

	// File B
	sourceB := filepath.Join(tmpDir, "sourceB.txt")
	keyB := filepath.Join(tmpDir, "keyB.key")
	require.NoError(t, os.WriteFile(sourceB, []byte("content of file B is different"), 0644))

	// Encryptor with a mock that provides different keys for each call
	callCount := 0
	mock := &mockVaultClient{
		generateKeyFunc: func() (*vault.DataKey, error) {
			callCount++
			if callCount == 1 {
				// Key for file A
				keyA := make([]byte, 32) // All zeros
				return &vault.DataKey{
					Plaintext:  keyA,
					Ciphertext: "vault:v1:key-a",
				}, nil
			}
			// Key for file B
			keyB := make([]byte, 32)
			for i := range keyB {
				keyB[i] = 1 // All ones
			}
			return &vault.DataKey{
				Plaintext:  keyB,
				Ciphertext: "vault:v1:key-b",
			}, nil
		},
		decryptKeyFunc: func(ciphertext string) (*vault.DataKey, error) {
			if ciphertext == "vault:v1:key-b" {
				keyB := make([]byte, 32)
				for i := range keyB {
					keyB[i] = 1 // All ones (Key B)
				}
				return &vault.DataKey{
					Plaintext: keyB,
				}, nil
			}
			return nil, assert.AnError
		},
	}
	encryptor := NewEncryptor(mock, nil)
	decryptor := NewDecryptor(mock, nil)

	// Encrypt file A -> creates encryptedA.enc and keyA.key
	encryptedKeyA, err := encryptor.EncryptFile(ctx, sourceA, encryptedA, nil)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyA, []byte(encryptedKeyA), 0644))

	// Encrypt file B to get a different key (keyB)
	encryptedKeyB, err := encryptor.EncryptFile(ctx, sourceB, filepath.Join(tmpDir, "encryptedB.enc"), nil)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyB, []byte(encryptedKeyB), 0644))

	// Attempt to decrypt file A with key B
	decryptedOut := filepath.Join(tmpDir, "decrypted.txt")
	err = decryptor.DecryptFile(ctx, encryptedA, keyB, decryptedOut, nil)

	// Should fail because the key is wrong
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")
}
