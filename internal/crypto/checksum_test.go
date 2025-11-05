package crypto

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateChecksum(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("test content for checksum calculation")

	err := os.WriteFile(testFile, testContent, 0644)
	require.NoError(t, err)

	// Calculate checksum
	checksum, err := CalculateChecksum(testFile)
	require.NoError(t, err)
	assert.NotEmpty(t, checksum)

	// SHA256 produces 64 hex characters (32 bytes * 2 chars/byte)
	assert.Len(t, checksum, 64)
}

func TestVerifyChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("test content for checksum verification")

	err := os.WriteFile(testFile, testContent, 0644)
	require.NoError(t, err)

	// Calculate correct checksum
	correctChecksum, err := CalculateChecksum(testFile)
	require.NoError(t, err)

	// Verify with correct checksum
	valid, err := VerifyChecksum(testFile, correctChecksum)
	require.NoError(t, err)
	assert.True(t, valid)

	// Verify with incorrect checksum
	valid, err = VerifyChecksum(testFile, "0000000000000000000000000000000000000000000000000000000000000000")
	require.NoError(t, err)
	assert.False(t, valid)
}

func TestSaveAndLoadChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	checksumFile := filepath.Join(tmpDir, "test.sha256")
	expectedChecksum := "abc123def456"

	// Save checksum
	err := SaveChecksum(expectedChecksum, checksumFile)
	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(checksumFile)
	require.NoError(t, err)

	// Load checksum
	loadedChecksum, err := LoadChecksum(checksumFile)
	require.NoError(t, err)
	assert.Equal(t, expectedChecksum, loadedChecksum)
}

func TestSaveChecksumError(t *testing.T) {
	// Try to save to an invalid path
	err := SaveChecksum("test", "/nonexistent/path/test.sha256")
	require.Error(t, err)
}

func TestLoadChecksumError(t *testing.T) {
	// Try to load from non-existent file
	_, err := LoadChecksum("/nonexistent/file.sha256")
	require.Error(t, err)
}
