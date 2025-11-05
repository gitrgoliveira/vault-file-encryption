package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestRewrapCmd_FlagValidation tests flag validation for the rewrap command
func TestRewrapCmd_FlagValidation(t *testing.T) {
	tests := []struct {
		name        string
		keyFile     string
		dir         string
		minVersion  int
		format      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "no flags provided",
			keyFile:     "",
			dir:         "",
			expectError: true,
			errorMsg:    "either --key-file or --dir must be specified",
		},
		{
			name:        "both key-file and dir provided",
			keyFile:     "test.key",
			dir:         "/path/to/keys",
			expectError: true,
			errorMsg:    "--key-file and --dir are mutually exclusive",
		},
		{
			name:        "invalid min-version (negative)",
			keyFile:     "test.key",
			minVersion:  -1,
			expectError: true,
			errorMsg:    "--min-version must be at least 1",
		},
		{
			name:        "invalid min-version (zero)",
			keyFile:     "test.key",
			minVersion:  0,
			expectError: true,
			errorMsg:    "--min-version must be at least 1",
		},
		{
			name:        "invalid format",
			keyFile:     "test.key",
			minVersion:  1,
			format:      "xml",
			expectError: true,
			errorMsg:    "--format must be one of: text, json, csv",
		},
		{
			name:        "valid key-file",
			keyFile:     "test.key",
			minVersion:  2,
			format:      "text",
			expectError: true, // Will fail because file doesn't exist, but flags are valid
			errorMsg:    "",   // Different error message about file not found
		},
		{
			name:        "valid dir",
			dir:         "/path/to/keys",
			minVersion:  1,
			format:      "json",
			expectError: true, // Will fail because dir doesn't exist, but flags are valid
			errorMsg:    "",   // Different error message about dir not found
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runRewrap(tt.keyFile, tt.dir, false, false, tt.minVersion, true, tt.format)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestKeyVersionsCmd_FlagValidation tests flag validation for the key-versions command
func TestKeyVersionsCmd_FlagValidation(t *testing.T) {
	tests := []struct {
		name        string
		keyFile     string
		dir         string
		format      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "no flags provided",
			keyFile:     "",
			dir:         "",
			expectError: true,
			errorMsg:    "either --key-file or --dir must be specified",
		},
		{
			name:        "both key-file and dir provided",
			keyFile:     "test.key",
			dir:         "/path/to/keys",
			expectError: true,
			errorMsg:    "--key-file and --dir are mutually exclusive",
		},
		{
			name:        "invalid format",
			keyFile:     "test.key",
			format:      "yaml",
			expectError: true,
			errorMsg:    "--format must be one of: text, json, csv",
		},
		{
			name:        "valid key-file",
			keyFile:     "test.key",
			format:      "text",
			expectError: true, // Will fail because file doesn't exist, but flags are valid
			errorMsg:    "",   // Different error message
		},
		{
			name:        "valid dir",
			dir:         "/path/to/keys",
			format:      "csv",
			expectError: true, // Will fail because dir doesn't exist, but flags are valid
			errorMsg:    "",   // Different error message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runKeyVersions(tt.keyFile, tt.dir, false, tt.format)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestRewrapCmd_FormatCaseInsensitive tests that format flag is case-insensitive
func TestRewrapCmd_FormatCaseInsensitive(t *testing.T) {
	formats := []string{"TEXT", "Json", "CSV", "tExT", "JsOn", "CsV"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			// This will fail due to missing key file, but should not fail due to format validation
			err := runRewrap("test.key", "", false, false, 1, true, format)

			if err == nil {
				t.Error("expected error (file not found), got none")
				return
			}

			// Should NOT contain format error
			if strings.Contains(err.Error(), "--format must be one of") {
				t.Errorf("format validation failed for %q: %v", format, err)
			}
		})
	}
}

// TestKeyVersionsCmd_FormatCaseInsensitive tests that format flag is case-insensitive
func TestKeyVersionsCmd_FormatCaseInsensitive(t *testing.T) {
	formats := []string{"TEXT", "Json", "CSV", "tExT", "JsOn", "CsV"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			// This will fail due to missing key file, but should not fail due to format validation
			err := runKeyVersions("test.key", "", false, format)

			if err == nil {
				t.Error("expected error (file not found), got none")
				return
			}

			// Should NOT contain format error
			if strings.Contains(err.Error(), "--format must be one of") {
				t.Errorf("format validation failed for %q: %v", format, err)
			}
		})
	}
}

// TestRewrapCmd_NonExistentFile tests error handling for non-existent files
func TestRewrapCmd_NonExistentFile(t *testing.T) {
	err := runRewrap("/non/existent/file.key", "", false, false, 1, true, "text")

	if err == nil {
		t.Error("expected error for non-existent file, got none")
		return
	}

	// Should contain error about file/config issues
	if !strings.Contains(err.Error(), "failed") {
		t.Errorf("expected meaningful error message, got: %v", err)
	}
}

// TestKeyVersionsCmd_NonExistentFile tests error handling for non-existent files
func TestKeyVersionsCmd_NonExistentFile(t *testing.T) {
	err := runKeyVersions("/non/existent/file.key", "", false, "text")

	if err == nil {
		t.Error("expected error for non-existent file, got none")
		return
	}

	// Should contain error about file/config issues
	if !strings.Contains(err.Error(), "failed") {
		t.Errorf("expected meaningful error message, got: %v", err)
	}
}

// TestRewrapCmd_InvalidDirectory tests error handling for invalid directory
func TestRewrapCmd_InvalidDirectory(t *testing.T) {
	err := runRewrap("", "/non/existent/directory", false, false, 1, true, "text")

	if err == nil {
		t.Error("expected error for non-existent directory, got none")
		return
	}

	// Should contain error about directory or scanner
	errMsg := err.Error()
	if !strings.Contains(errMsg, "failed") && !strings.Contains(errMsg, "directory") {
		t.Errorf("expected error about directory, got: %v", err)
	}
}

// TestKeyVersionsCmd_InvalidDirectory tests error handling for invalid directory
func TestKeyVersionsCmd_InvalidDirectory(t *testing.T) {
	err := runKeyVersions("", "/non/existent/directory", false, "text")

	if err == nil {
		t.Error("expected error for non-existent directory, got none")
		return
	}

	// Should contain error about directory or scanner
	errMsg := err.Error()
	if !strings.Contains(errMsg, "failed") && !strings.Contains(errMsg, "directory") {
		t.Errorf("expected error about directory, got: %v", err)
	}
}

// TestCobraCommandStructure tests that cobra commands are properly structured
func TestCobraCommandStructure(t *testing.T) {
	tests := []struct {
		name    string
		cmdFunc func() *cobra.Command
		wantUse string
	}{
		{
			name:    "rewrap command",
			cmdFunc: rewrapCmd,
			wantUse: "rewrap",
		},
		{
			name:    "key-versions command",
			cmdFunc: keyVersionsCmd,
			wantUse: "key-versions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.cmdFunc()

			if cmd.Use != tt.wantUse {
				t.Errorf("command Use = %q, want %q", cmd.Use, tt.wantUse)
			}

			if cmd.Short == "" {
				t.Error("command Short description is empty")
			}

			if cmd.Long == "" {
				t.Error("command Long description is empty")
			}

			if cmd.Example == "" {
				t.Error("command Example is empty")
			}

			if cmd.RunE == nil {
				t.Error("command RunE is nil")
			}
		})
	}
}

// TestRewrapCmd_Flags tests that rewrap command has all expected flags
func TestRewrapCmd_Flags(t *testing.T) {
	cmd := rewrapCmd()

	expectedFlags := []string{
		"key-file",
		"dir",
		"recursive",
		"dry-run",
		"min-version",
		"backup",
		"format",
	}

	for _, flagName := range expectedFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("expected flag %q not found", flagName)
		}
	}
}

// TestKeyVersionsCmd_Flags tests that key-versions command has all expected flags
func TestKeyVersionsCmd_Flags(t *testing.T) {
	cmd := keyVersionsCmd()

	expectedFlags := []string{
		"key-file",
		"dir",
		"recursive",
		"format",
	}

	for _, flagName := range expectedFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("expected flag %q not found", flagName)
		}
	}

	// Verify that rewrap-specific flags are NOT present
	notExpectedFlags := []string{"dry-run", "min-version", "backup"}
	for _, flagName := range notExpectedFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag != nil {
			t.Errorf("unexpected flag %q found in key-versions command", flagName)
		}
	}
}

// TestCobraCommandHelp tests that help text can be generated without errors
func TestCobraCommandHelp(t *testing.T) {
	commands := []struct {
		name string
		cmd  func() *cobra.Command
	}{
		{"rewrap", rewrapCmd},
		{"key-versions", keyVersionsCmd},
	}

	for _, tc := range commands {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.cmd()

			// Capture help output
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)

			// Generate help
			err := cmd.Help()
			if err != nil {
				t.Errorf("failed to generate help: %v", err)
			}

			help := buf.String()
			if help == "" {
				t.Error("help output is empty")
			}

			// Check that help contains essential elements
			if !strings.Contains(help, "Usage:") {
				t.Error("help missing Usage section")
			}
			if !strings.Contains(help, "Flags:") {
				t.Error("help missing Flags section")
			}
			if !strings.Contains(help, "Examples:") {
				t.Error("help missing Examples section")
			}
		})
	}
}

// TestRewrapCmd_WithRealFiles tests rewrap command with actual test files
func TestRewrapCmd_WithRealFiles(t *testing.T) {
	// Skip if config file doesn't exist (unit test environment)
	if _, err := os.Stat("../../configs/examples/example-enterprise.hcl"); os.IsNotExist(err) {
		t.Skip("Skipping test - config file not available")
	}

	// Create temp directory with test key file
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "test.key")

	// Write a valid-looking key file (vault:v1:... format)
	content := "vault:v1:dGVzdC1lbmNyeXB0ZWQta2V5LWRhdGE="
	if err := os.WriteFile(keyFile, []byte(content), 0600); err != nil {
		t.Fatalf("failed to create test key file: %v", err)
	}

	// Set config file (this will fail due to Vault connectivity, but tests flag handling)
	oldConfigFile := configFile
	configFile = "../../configs/examples/example-enterprise.hcl"
	defer func() { configFile = oldConfigFile }()

	// Test with valid flags - will fail on Vault connection but flag validation should pass
	err := runRewrap(keyFile, "", false, false, 2, true, "text")

	// Should get error about Vault or config, not about flag validation
	if err != nil {
		if strings.Contains(err.Error(), "must be specified") ||
			strings.Contains(err.Error(), "are mutually exclusive") ||
			strings.Contains(err.Error(), "--format must be") {
			t.Errorf("got flag validation error, expected Vault/config error: %v", err)
		}
	}
}

// TestKeyVersionsCmd_WithRealFiles tests key-versions command with actual test files
func TestKeyVersionsCmd_WithRealFiles(t *testing.T) {
	// Skip if config file doesn't exist
	if _, err := os.Stat("../../configs/examples/example-enterprise.hcl"); os.IsNotExist(err) {
		t.Skip("Skipping test - config file not available")
	}

	// Create temp directory with test key files
	tmpDir := t.TempDir()
	keyFile1 := filepath.Join(tmpDir, "test1.key")
	keyFile2 := filepath.Join(tmpDir, "test2.key")

	// Write valid-looking key files
	if err := os.WriteFile(keyFile1, []byte("vault:v1:dGVzdDE="), 0600); err != nil {
		t.Fatalf("failed to create test key file 1: %v", err)
	}
	if err := os.WriteFile(keyFile2, []byte("vault:v2:dGVzdDI="), 0600); err != nil {
		t.Fatalf("failed to create test key file 2: %v", err)
	}

	// Set config file
	oldConfigFile := configFile
	configFile = "../../configs/examples/example-enterprise.hcl"
	defer func() { configFile = oldConfigFile }()

	// Test directory scan - will fail on Vault connection but should handle files correctly
	err := runKeyVersions("", tmpDir, false, "text")

	// Should get error about Vault or config, not about flag validation
	if err != nil {
		if strings.Contains(err.Error(), "must be specified") ||
			strings.Contains(err.Error(), "are mutually exclusive") ||
			strings.Contains(err.Error(), "--format must be") {
			t.Errorf("got flag validation error, expected Vault/config error: %v", err)
		}
	}
}
