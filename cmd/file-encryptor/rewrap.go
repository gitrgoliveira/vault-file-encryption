package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gitrgoliveira/vault-file-encryption/internal/config"
	"github.com/gitrgoliveira/vault-file-encryption/internal/logger"
	"github.com/gitrgoliveira/vault-file-encryption/internal/rewrap"
	"github.com/gitrgoliveira/vault-file-encryption/internal/vault"
	"github.com/spf13/cobra"
)

// rewrapCmd re-wraps encrypted data keys to a newer version
func rewrapCmd() *cobra.Command {
	var (
		keyFile      string
		directory    string
		recursive    bool
		dryRun       bool
		minVersion   int
		enableBackup bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "rewrap",
		Short: "Re-wrap encrypted data keys to newer versions",
		Long: `Re-wraps encrypted data keys (.key files) to use a newer version of the Vault Transit encryption key.
This operation is necessary when you want to rotate to a newer key version for enhanced security.

The rewrap operation:
1. Scans for .key files in the specified directory
2. Checks if each key is below the minimum version
3. Requests Vault to re-wrap the key to the latest version
4. Atomically updates the .key file with the new ciphertext
5. Optionally creates backups before modification

The encrypted files (.enc) do not need to be re-encrypted, only the .key files are updated.`,
		Example: `  # Re-wrap a single key file
  file-encryptor rewrap --key-file data.txt.key --min-version 2

  # Re-wrap all keys in a directory (non-recursive)
  file-encryptor rewrap --dir /path/to/keys --min-version 2

  # Re-wrap all keys recursively with backups
  file-encryptor rewrap --dir /path/to/keys --recursive --backup --min-version 2

  # Dry-run to see what would be re-wrapped
  file-encryptor rewrap --dir /path/to/keys --recursive --dry-run --min-version 2

  # Output results as JSON
  file-encryptor rewrap --dir /path/to/keys --min-version 2 --format json

  # Output results as CSV
  file-encryptor rewrap --dir /path/to/keys --min-version 2 --format csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRewrap(keyFile, directory, recursive, dryRun, minVersion, enableBackup, outputFormat)
		},
	}

	cmd.Flags().StringVarP(&keyFile, "key-file", "k", "", "Single key file to re-wrap")
	cmd.Flags().StringVarP(&directory, "dir", "d", "", "Directory containing key files to re-wrap")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively scan directory for key files")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be re-wrapped without making changes")
	cmd.Flags().IntVarP(&minVersion, "min-version", "m", 1, "Minimum key version (re-wrap keys below this version)")
	cmd.Flags().BoolVarP(&enableBackup, "backup", "b", true, "Create backups before re-wrapping (enabled by default)")
	cmd.Flags().StringVarP(&outputFormat, "format", "f", "text", "Output format: text, json, csv")

	return cmd
}

func runRewrap(keyFile, directory string, recursive, dryRun bool, minVersion int, enableBackup bool, outputFormat string) error {
	// Validate flags
	if keyFile == "" && directory == "" {
		return fmt.Errorf("either --key-file or --dir must be specified")
	}
	if keyFile != "" && directory != "" {
		return fmt.Errorf("--key-file and --dir are mutually exclusive")
	}
	if minVersion < 1 {
		return fmt.Errorf("--min-version must be at least 1")
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

	if dryRun {
		log.Info("Running in dry-run mode - no changes will be made")
	}

	// Load configuration (only Vault settings needed)
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate Vault config
	if cfg.Vault.AgentAddress == "" || cfg.Vault.TransitMount == "" || cfg.Vault.KeyName == "" {
		return fmt.Errorf("vault configuration is incomplete (agent_address, transit_mount, key_name required)")
	}

	// Create Vault client
	vaultClient, err := vault.NewClient(&vault.Config{
		AgentAddress: cfg.Vault.AgentAddress,
		TransitMount: cfg.Vault.TransitMount,
		KeyName:      cfg.Vault.KeyName,
		Timeout:      cfg.Vault.RequestTimeout,
	})
	if err != nil {
		return fmt.Errorf("failed to create vault client: %w", err)
	}
	defer func() { _ = vaultClient.Close() }()

	// Create rewrapper
	rewrapper, err := rewrap.NewRewrapper(rewrap.RewrapOptions{
		VaultClient:  vaultClient,
		MinVersion:   minVersion,
		DryRun:       dryRun,
		CreateBackup: enableBackup,
		BackupSuffix: ".bak",
		Logger:       log,
	})
	if err != nil {
		return fmt.Errorf("failed to create rewrapper: %w", err)
	}

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

	// Create reporter
	reporter := rewrap.NewReporter()

	// Create context
	ctx := context.Background()

	// Re-wrap files
	results, err := rewrapper.RewrapBatch(ctx, files)
	if err != nil {
		return fmt.Errorf("rewrap batch failed: %w", err)
	}

	// Add results to reporter
	reporter.AddResults(results)

	// Output results
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
		// Text format includes detailed file list
		if err := reporter.WriteText(os.Stdout, true); err != nil {
			return fmt.Errorf("failed to write text output: %w", err)
		}
	}

	// Determine exit code based on results
	stats := reporter.GetStatistics()
	if stats.Failed > 0 {
		// Some failures occurred
		if stats.Successful > 0 {
			log.Error("Rewrap completed with failures", "successful", stats.Successful, "failed", stats.Failed)
			os.Exit(1) // Partial success
		} else {
			log.Error("Rewrap failed completely", "failed", stats.Failed)
			os.Exit(2) // Complete failure
		}
	}

	if dryRun {
		log.Info("Dry-run completed", "total", stats.TotalFiles, "would_rewrap", stats.Successful, "would_skip", stats.Skipped)
	} else {
		log.Info("Rewrap completed successfully", "total", stats.TotalFiles, "rewrapped", stats.Successful, "skipped", stats.Skipped)
	}

	return nil
}
