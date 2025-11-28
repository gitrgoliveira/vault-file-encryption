package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/gitrgoliveira/vault-file-encryption/internal/logger"
	"github.com/gitrgoliveira/vault-file-encryption/internal/rewrap"
	"github.com/gitrgoliveira/vault-file-encryption/internal/vault"
	"github.com/spf13/cobra"
)

// keyVersionsCmd displays statistics about key file versions without rewrapping
func keyVersionsCmd() *cobra.Command {
	var (
		keyFile      string
		directory    string
		recursive    bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "key-versions",
		Short: "Display encryption key version statistics",
		Long: `Displays statistics about the Vault Transit encryption key versions used by encrypted data keys (.key files).

This command scans for .key files and reports their version distribution without making any changes.
Use this to audit your key versions before performing a rewrap operation.

The command shows:
- Total number of key files found
- Distribution of keys by version number
- Detailed list of files (optional)

No modifications are made to any files.`,
		Example: `  # Show version statistics for a single key file
  file-encryptor key-versions --key-file data.txt.key

  # Show statistics for all keys in a directory
  file-encryptor key-versions --dir /path/to/keys

  # Recursively scan directory
  file-encryptor key-versions --dir /path/to/keys --recursive

  # Output as JSON
  file-encryptor key-versions --dir /path/to/keys --format json

  # Output as CSV for spreadsheet analysis
  file-encryptor key-versions --dir /path/to/keys --recursive --format csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKeyVersions(keyFile, directory, recursive, outputFormat)
		},
	}

	cmd.Flags().StringVarP(&keyFile, "key-file", "k", "", "Single key file to check")
	cmd.Flags().StringVarP(&directory, "dir", "d", "", "Directory containing key files")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively scan directory for key files")
	cmd.Flags().StringVarP(&outputFormat, "format", "f", "text", "Output format: text, json, csv")

	return cmd
}

func runKeyVersions(keyFile, directory string, recursive bool, outputFormat string) error {
	// Validate flags
	if keyFile == "" && directory == "" {
		return fmt.Errorf("either --key-file or --dir must be specified")
	}
	if keyFile != "" && directory != "" {
		return fmt.Errorf("--key-file and --dir are mutually exclusive")
	}

	// Validate output format
	outputFormat = strings.ToLower(outputFormat)
	if outputFormat != "text" && outputFormat != "json" && outputFormat != "csv" {
		return fmt.Errorf("--format must be one of: text, json, csv")
	}

	// Initialize logger
	log, err := logger.New(logLevel, logOutput)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	// Scan for key files
	var files []string
	if keyFile != "" {
		// Single file mode
		files = []string{keyFile}
	} else {
		// Directory scan mode
		scanner, err := rewrap.NewScanner(rewrap.ScanOptions{
			Directory: directory,
			Recursive: recursive,
		})
		if err != nil {
			return fmt.Errorf("failed to create scanner: %w", err)
		}

		scanResult, err := scanner.Scan()
		if err != nil {
			return fmt.Errorf("failed to scan directory: %w", err)
		}

		files = scanResult.Files

		if len(files) == 0 {
			log.Info("No .key files found", "directory", directory, "recursive", recursive)
			return nil
		}

		log.Info("Found key files", "count", len(files), "directory", directory, "recursive", recursive)
	}

	// Create reporter for collecting results
	reporter := rewrap.NewReporter()

	// Process each file to get version info
	for _, filePath := range files {
		// Read key file
		ciphertext, err := os.ReadFile(filePath) // #nosec G304 - user-provided key file path
		if err != nil {
			log.Error("Failed to read key file", "file", filePath, "error", err)
			reporter.AddResult(&vault.RewrapResult{
				FilePath: filePath,
				Error:    fmt.Errorf("failed to read file: %w", err),
			})
			continue
		}

		// Get version info without calling Vault
		versionInfo, err := vault.GetKeyVersionInfo(filePath, string(ciphertext), 0)
		if err != nil {
			log.Error("Failed to get version info", "file", filePath, "error", err)
			reporter.AddResult(&vault.RewrapResult{
				FilePath: filePath,
				Error:    err,
			})
			continue
		}

		// Add to reporter as a "skipped" result (no rewrap performed)
		reporter.AddResult(&vault.RewrapResult{
			FilePath:      filePath,
			OldVersion:    versionInfo.Version,
			NewVersion:    0, // No rewrap performed
			BackupCreated: false,
			Error:         nil,
		})

		log.Debug("Key file version", "file", filePath, "version", versionInfo.Version)
	}

	// Output results (no need to query Vault for latest version in this command)
	switch outputFormat {
	case "json":
		if err := reporter.WriteJSON(os.Stdout, true); err != nil {
			return fmt.Errorf("failed to write JSON output: %w", err)
		}
	case "csv":
		if err := reporter.WriteCSV(os.Stdout); err != nil {
			return fmt.Errorf("failed to write CSV output: %w", err)
		}
	default: // text
		// Include detailed file list for text format
		if err := reporter.WriteText(os.Stdout, true); err != nil {
			return fmt.Errorf("failed to write text output: %w", err)
		}
	}

	// Get statistics for summary logging
	stats := reporter.GetStatistics()
	log.Info("Key version scan complete",
		"total_files", stats.TotalFiles,
		"successful", stats.Successful+stats.Skipped, // All non-errors
		"failed", stats.Failed)

	// Exit with error if any files failed
	if stats.Failed > 0 {
		return fmt.Errorf("%d file(s) failed to process", stats.Failed)
	}

	return nil
}
