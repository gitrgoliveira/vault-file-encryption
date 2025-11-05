package rewrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScanOptions configures the key file scanner.
type ScanOptions struct {
	Directory string // Root directory to scan
	Recursive bool   // Whether to scan subdirectories recursively
}

// ScanResult represents the outcome of scanning a directory for key files.
type ScanResult struct {
	Files []string // List of discovered .key file paths
	Count int      // Number of files found
	Error error    // Error if scan failed
}

// Scanner finds .key files in a directory structure.
type Scanner struct {
	options ScanOptions
}

// NewScanner creates a new key file scanner.
func NewScanner(options ScanOptions) (*Scanner, error) {
	if options.Directory == "" {
		return nil, fmt.Errorf("directory cannot be empty")
	}

	// Verify directory exists
	info, err := os.Stat(options.Directory)
	if err != nil {
		return nil, fmt.Errorf("failed to access directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", options.Directory)
	}

	return &Scanner{
		options: options,
	}, nil
}

// Scan searches for .key files according to the configured options.
func (s *Scanner) Scan() (*ScanResult, error) {
	result := &ScanResult{
		Files: make([]string, 0),
	}

	// Walk the directory tree
	err := filepath.Walk(s.options.Directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// If we encounter an error accessing a file/directory, log and continue
			return nil
		}

		// Skip directories unless we're in recursive mode
		if info.IsDir() {
			// Always process the root directory
			if path == s.options.Directory {
				return nil
			}
			// In non-recursive mode, skip subdirectories
			if !s.options.Recursive {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file has .key extension
		if strings.HasSuffix(info.Name(), ".key") {
			result.Files = append(result.Files, path)
		}

		return nil
	})

	if err != nil {
		result.Error = fmt.Errorf("scan failed: %w", err)
		return result, err
	}

	result.Count = len(result.Files)
	return result, nil
}

// ScanSingleFile validates and returns a single .key file path.
// This is a convenience function for processing a single file.
func ScanSingleFile(filePath string) (*ScanResult, error) {
	result := &ScanResult{
		Files: make([]string, 0, 1),
	}

	// Verify file exists
	info, err := os.Stat(filePath)
	if err != nil {
		result.Error = fmt.Errorf("failed to access file: %w", err)
		return result, result.Error
	}

	if info.IsDir() {
		result.Error = fmt.Errorf("path is a directory, not a file: %s", filePath)
		return result, result.Error
	}

	// Verify .key extension
	if !strings.HasSuffix(filePath, ".key") {
		result.Error = fmt.Errorf("file must have .key extension: %s", filePath)
		return result, result.Error
	}

	result.Files = append(result.Files, filePath)
	result.Count = 1
	return result, nil
}
