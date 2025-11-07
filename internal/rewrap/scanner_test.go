package rewrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewScanner(t *testing.T) {
	tests := []struct {
		name        string
		options     ScanOptions
		expectError bool
		errorMsg    string
	}{
		{
			name: "empty directory",
			options: ScanOptions{
				Directory: "",
			},
			expectError: true,
			errorMsg:    "directory cannot be empty",
		},
		{
			name: "non-existent directory",
			options: ScanOptions{
				Directory: "/nonexistent/path",
			},
			expectError: true,
			errorMsg:    "failed to access directory",
		},
		{
			name: "path is file not directory",
			options: ScanOptions{
				Directory: func() string {
					tmpFile, _ := os.CreateTemp("", "testfile")
					defer func() { _ = tmpFile.Close() }()
					return tmpFile.Name()
				}(),
			},
			expectError: true,
			errorMsg:    "path is not a directory",
		},
		{
			name: "valid directory",
			options: ScanOptions{
				Directory: os.TempDir(),
				Recursive: false,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner, err := NewScanner(tt.options)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, scanner)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, scanner)
			}
		})
	}
}

func TestScanner_Scan(t *testing.T) {
	// Create temporary directory structure for testing
	tmpDir := t.TempDir()

	// Create test files
	createTestFile := func(path string) {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(fullPath, []byte("test"), 0644))
	}

	// Create directory structure:
	// tmpDir/
	//   file1.key
	//   file2.txt
	//   subdir1/
	//     file3.key
	//     file4.enc
	//   subdir2/
	//     nested/
	//       file5.key
	createTestFile("file1.key")
	createTestFile("file2.txt")
	createTestFile("subdir1/file3.key")
	createTestFile("subdir1/file4.enc")
	createTestFile("subdir2/nested/file5.key")

	tests := []struct {
		name          string
		directory     string
		recursive     bool
		expectedCount int
		expectedFiles []string
	}{
		{
			name:          "non-recursive scan finds root files only",
			directory:     tmpDir,
			recursive:     false,
			expectedCount: 1,
			expectedFiles: []string{"file1.key"},
		},
		{
			name:          "recursive scan finds all key files",
			directory:     tmpDir,
			recursive:     true,
			expectedCount: 3,
			expectedFiles: []string{"file1.key", "subdir1/file3.key", "subdir2/nested/file5.key"},
		},
		{
			name:          "scan subdirectory non-recursively",
			directory:     filepath.Join(tmpDir, "subdir1"),
			recursive:     false,
			expectedCount: 1,
			expectedFiles: []string{"file3.key"},
		},
		{
			name:          "scan empty subdirectory",
			directory:     filepath.Join(tmpDir, "subdir2/nested"),
			recursive:     false,
			expectedCount: 1,
			expectedFiles: []string{"file5.key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner, err := NewScanner(ScanOptions{
				Directory: tt.directory,
				Recursive: tt.recursive,
			})
			require.NoError(t, err)

			result, err := scanner.Scan()
			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedCount, result.Count)
			assert.Equal(t, tt.expectedCount, len(result.Files))

			// Verify expected files are present (convert to relative paths)
			relativeFiles := make([]string, len(result.Files))
			for i, f := range result.Files {
				rel, err := filepath.Rel(tt.directory, f)
				require.NoError(t, err)
				// Normalize path separators for cross-platform compatibility
				relativeFiles[i] = filepath.ToSlash(rel)
			}

			for _, expectedFile := range tt.expectedFiles {
				assert.Contains(t, relativeFiles, expectedFile, "Expected to find %s", expectedFile)
			}
		})
	}
}

func TestScanner_Scan_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	scanner, err := NewScanner(ScanOptions{
		Directory: tmpDir,
		Recursive: true,
	})
	require.NoError(t, err)

	result, err := scanner.Scan()
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.Count)
	assert.Equal(t, 0, len(result.Files))
}

func TestScanSingleFile(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		setup       func() string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid key file",
			setup: func() string {
				path := filepath.Join(tmpDir, "valid.key")
				require.NoError(t, os.WriteFile(path, []byte("test"), 0644))
				return path
			},
			expectError: false,
		},
		{
			name: "non-existent file",
			setup: func() string {
				return filepath.Join(tmpDir, "nonexistent.key")
			},
			expectError: true,
			errorMsg:    "failed to access file",
		},
		{
			name: "directory instead of file",
			setup: func() string {
				dir := filepath.Join(tmpDir, "testdir")
				require.NoError(t, os.Mkdir(dir, 0755))
				return dir
			},
			expectError: true,
			errorMsg:    "path is a directory",
		},
		{
			name: "wrong file extension",
			setup: func() string {
				path := filepath.Join(tmpDir, "wrong.txt")
				require.NoError(t, os.WriteFile(path, []byte("test"), 0644))
				return path
			},
			expectError: true,
			errorMsg:    "must have .key extension",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := tt.setup()

			result, err := ScanSingleFile(filePath)
			require.NotNil(t, result)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Equal(t, 0, result.Count)
				assert.Equal(t, 0, len(result.Files))
			} else {
				require.NoError(t, err)
				assert.Equal(t, 1, result.Count)
				assert.Equal(t, 1, len(result.Files))
				assert.Equal(t, filePath, result.Files[0])
			}
		})
	}
}
