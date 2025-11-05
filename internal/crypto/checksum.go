package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// CalculateChecksum calculates SHA256 checksum of a file
func CalculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath) // #nosec G304 - intentional file encryption tool
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close file: %w", closeErr)
		}
	}()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	checksum := hex.EncodeToString(hash.Sum(nil))
	return checksum, nil
}

// VerifyChecksum verifies that a file's checksum matches the expected value
func VerifyChecksum(filePath, expectedChecksum string) (bool, error) {
	actualChecksum, err := CalculateChecksum(filePath)
	if err != nil {
		return false, err
	}

	return actualChecksum == expectedChecksum, nil
}

// SaveChecksum saves a checksum to a file
func SaveChecksum(checksum, checksumPath string) error {
	if err := os.WriteFile(checksumPath, []byte(checksum), 0600); err != nil { // #nosec G306 - checksum file
		return fmt.Errorf("failed to save checksum: %w", err)
	}
	return nil
}

// LoadChecksum loads a checksum from a file
func LoadChecksum(checksumPath string) (string, error) {
	data, err := os.ReadFile(checksumPath) // #nosec G304 - intentional file encryption tool
	if err != nil {
		return "", fmt.Errorf("failed to load checksum: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}
