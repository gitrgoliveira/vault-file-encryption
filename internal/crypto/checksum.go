package crypto

import (
	"fmt"
	"os"
	"strings"

	fileencrypt "github.com/gitrgoliveira/go-fileencrypt"
)

// CalculateChecksum calculates SHA256 checksum of a file
func CalculateChecksum(filePath string) (string, error) {
	checksum, err := fileencrypt.CalculateChecksumHex(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}
	return checksum, nil
}

// VerifyChecksum verifies that a file's checksum matches the expected value
func VerifyChecksum(filePath, expectedChecksum string) (bool, error) {
	return fileencrypt.VerifyChecksumHex(filePath, expectedChecksum)
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
