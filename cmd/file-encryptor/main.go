package main

import (
	"context"
	"fmt"
	"os"

	"github.com/gitrgoliveira/vault-file-encryption/internal/config"
	"github.com/gitrgoliveira/vault-file-encryption/internal/crypto"
	"github.com/gitrgoliveira/vault-file-encryption/internal/logger"
	"github.com/gitrgoliveira/vault-file-encryption/internal/service"
	"github.com/gitrgoliveira/vault-file-encryption/internal/vault"
	"github.com/gitrgoliveira/vault-file-encryption/internal/version"
	"github.com/spf13/cobra"
)

var (
	configFile string
	logLevel   string
	logOutput  string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "file-encryptor",
		Short: "File encryption service using HashiCorp Vault",
		Long: `A file watcher that encrypts files using Vault Transit Engine
and stores them in a destination folder with envelope encryption.

Can also be used for one-off file encryption/decryption.`,
		Version: version.FullVersion(),
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.hcl", "Configuration file path")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, error)")
	rootCmd.PersistentFlags().StringVar(&logOutput, "log-output", "stdout", "Log output (stdout, stderr, or file path)")

	// Add subcommands
	rootCmd.AddCommand(watchCmd())
	rootCmd.AddCommand(encryptCmd())
	rootCmd.AddCommand(decryptCmd())
	rootCmd.AddCommand(rewrapCmd())
	rootCmd.AddCommand(keyVersionsCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// watchCmd runs the file watcher service
func watchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Run as a service watching directories for files",
		Long:  `Starts the file watcher service that continuously monitors directories for new files to encrypt/decrypt.`,
		RunE:  runWatch,
	}
	return cmd
}

// encryptCmd encrypts a single file
func encryptCmd() *cobra.Command {
	var (
		inputFile  string
		outputFile string
		keyFile    string
		checksum   bool
		chunkSize  string
	)

	cmd := &cobra.Command{
		Use:   "encrypt",
		Short: "Encrypt a single file",
		Long:  `Encrypts a single file using Vault Transit Engine with envelope encryption.`,
		Example: `  # Encrypt a file
  file-encryptor encrypt -i data.txt -o data.txt.enc
  
  # Encrypt with custom key file location
  file-encryptor encrypt -i data.txt -o data.txt.enc -k my-data.key
  
  # Encrypt with checksum
  file-encryptor encrypt -i data.txt -o data.txt.enc --checksum
  
  # Encrypt with custom chunk size
  file-encryptor encrypt -i large.db -o large.db.enc --chunk-size 5MB`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEncrypt(inputFile, outputFile, keyFile, checksum, chunkSize)
		},
	}

	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "Input file to encrypt")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output encrypted file")
	cmd.Flags().StringVarP(&keyFile, "key-file", "k", "", "Output key file (default: output.key)")
	cmd.Flags().BoolVar(&checksum, "checksum", false, "Calculate and save checksum")
	cmd.Flags().StringVar(&chunkSize, "chunk-size", "", "Chunk size for encryption (e.g., 2MB, 512KB) - overrides config")

	_ = cmd.MarkFlagRequired("input")
	_ = cmd.MarkFlagRequired("output")

	return cmd
}

// decryptCmd decrypts a single file
func decryptCmd() *cobra.Command {
	var (
		inputFile      string
		keyFile        string
		outputFile     string
		verifyChecksum bool
	)

	cmd := &cobra.Command{
		Use:   "decrypt",
		Short: "Decrypt a single file",
		Long:  `Decrypts a single file that was encrypted with Vault Transit Engine.`,
		Example: `  # Decrypt a file
  file-encryptor decrypt -i data.txt.enc -k data.txt.key -o data.txt
  
  # Decrypt with checksum verification
  file-encryptor decrypt -i data.txt.enc -k data.txt.key -o data.txt --verify-checksum`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDecrypt(inputFile, keyFile, outputFile, verifyChecksum)
		},
	}

	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "Encrypted file to decrypt (required)")
	cmd.Flags().StringVarP(&keyFile, "key", "k", "", "Key file (required)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output decrypted file (required)")
	cmd.Flags().BoolVar(&verifyChecksum, "verify-checksum", false, "Verify SHA256 checksum if available")

	_ = cmd.MarkFlagRequired("input")
	_ = cmd.MarkFlagRequired("key")
	_ = cmd.MarkFlagRequired("output")

	return cmd
}

func runWatch(cmd *cobra.Command, args []string) error {
	// Create signal handler
	sigChan := setupSignalHandler()

	// Create and start the service
	svc, err := service.New(&service.Config{
		ConfigFile: configFile,
		SignalChan: sigChan,
	})
	if err != nil {
		return err
	}
	defer func() { _ = svc.Close() }()

	// Run the service
	ctx := context.Background()
	return svc.Run(ctx, sigChan, isReloadSignal, isShutdownSignal)
}

func runEncrypt(inputFile, outputFile, keyFile string, calculateChecksum bool, chunkSizeStr string) error {
	// Initialize logger (use flags, not config file)
	log, err := logger.New(logLevel, logOutput)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	log.Info("Encrypting file", "input", inputFile, "output", outputFile)

	// Verify input file exists
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return fmt.Errorf("input file does not exist: %s", inputFile)
	}

	// Load configuration (only Vault settings are needed for CLI mode)
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Only validate Vault config for CLI mode
	if cfg.Vault.AgentAddress == "" || cfg.Vault.TransitMount == "" || cfg.Vault.KeyName == "" {
		return fmt.Errorf("vault configuration is incomplete (agent_address, transit_mount, key_name required)")
	}

	// Create Vault client
	vaultClient, err := vault.NewClient(&vault.Config{
		AgentAddress: cfg.Vault.AgentAddress,
		TransitMount: cfg.Vault.TransitMount,
		KeyName:      cfg.Vault.KeyName,
		Timeout:      cfg.Vault.RequestTimeout.Duration(),
	})
	if err != nil {
		return fmt.Errorf("failed to create vault client: %w", err)
	}
	defer func() { _ = vaultClient.Close() }()

	// Determine chunk size (CLI flag overrides config)
	chunkSize := cfg.Encryption.ChunkSize
	if chunkSizeStr != "" {
		size, err := config.ParseSize(chunkSizeStr)
		if err != nil {
			return fmt.Errorf("invalid chunk size: %w", err)
		}
		chunkSize = size
		log.Info("Using custom chunk size", "chunk_size", config.FormatSize(chunkSize))
	}

	// Create encryptor
	encryptor := crypto.NewEncryptor(vaultClient, &crypto.EncryptorConfig{
		ChunkSize: chunkSize,
	})

	// Progress callback
	progressCallback := func(progress float64) {
		log.Info("Encryption progress", "file", inputFile, "progress", fmt.Sprintf("%.0f%%", progress))
	}

	// Create context for the operation
	ctx := context.Background()

	// Encrypt the file
	encryptedKey, err := encryptor.EncryptFile(ctx, inputFile, outputFile, progressCallback)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	// Determine key file path
	if keyFile == "" {
		keyFile = outputFile + ".key"
	}

	// Save the encrypted data key
	if err := os.WriteFile(keyFile, []byte(encryptedKey), 0600); err != nil {
		return fmt.Errorf("failed to save key file: %w", err)
	}

	log.Info("Encrypted data key saved", "key_file", keyFile)

	// Calculate and save checksum if requested
	if calculateChecksum {
		checksumPath := inputFile + ".sha256"
		checksum, err := crypto.CalculateChecksum(inputFile)
		if err != nil {
			return fmt.Errorf("failed to calculate checksum: %w", err)
		}

		if err := crypto.SaveChecksum(checksum, checksumPath); err != nil {
			return fmt.Errorf("failed to save checksum: %w", err)
		}

		log.Info("Checksum saved", "checksum_file", checksumPath, "checksum", checksum)
	}

	log.Info("File encrypted successfully",
		"input", inputFile,
		"output", outputFile,
		"key_file", keyFile)

	return nil
}

func runDecrypt(inputFile, keyFile, outputFile string, verifyChecksum bool) error {
	// Initialize logger (use flags, not config file)
	log, err := logger.New(logLevel, logOutput)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	log.Info("Decrypting file", "input", inputFile, "output", outputFile)

	// Verify input files exist
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return fmt.Errorf("encrypted file does not exist: %s", inputFile)
	}
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		return fmt.Errorf("key file does not exist: %s", keyFile)
	}

	// Load configuration (only Vault settings are needed for CLI mode)
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Only validate Vault config for CLI mode
	if cfg.Vault.AgentAddress == "" || cfg.Vault.TransitMount == "" || cfg.Vault.KeyName == "" {
		return fmt.Errorf("vault configuration is incomplete (agent_address, transit_mount, key_name required)")
	}

	// Create Vault client
	vaultClient, err := vault.NewClient(&vault.Config{
		AgentAddress: cfg.Vault.AgentAddress,
		TransitMount: cfg.Vault.TransitMount,
		KeyName:      cfg.Vault.KeyName,
		Timeout:      cfg.Vault.RequestTimeout.Duration(),
	})
	if err != nil {
		return fmt.Errorf("failed to create vault client: %w", err)
	}
	defer func() { _ = vaultClient.Close() }()

	// Create decryptor with config chunk size
	decryptor := crypto.NewDecryptor(vaultClient, &crypto.EncryptorConfig{
		ChunkSize: cfg.Encryption.ChunkSize,
	})

	// Progress callback
	progressCallback := func(progress float64) {
		log.Info("Decryption progress", "file", inputFile, "progress", fmt.Sprintf("%.0f%%", progress))
	}

	// Create context for the operation
	ctx := context.Background()

	// Decrypt the file
	if err := decryptor.DecryptFile(ctx, inputFile, keyFile, outputFile, progressCallback); err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}

	// Verify checksum if requested
	if verifyChecksum {
		// Determine original file name from output file
		// Assume checksum file is output + .sha256
		checksumPath := outputFile + ".sha256"

		// Check if checksum file exists
		if _, err := os.Stat(checksumPath); err == nil {
			log.Info("Verifying checksum", "checksum_file", checksumPath)

			expectedChecksum, err := crypto.LoadChecksum(checksumPath)
			if err != nil {
				return fmt.Errorf("failed to load checksum: %w", err)
			}

			valid, err := crypto.VerifyChecksum(outputFile, expectedChecksum)
			if err != nil {
				return fmt.Errorf("failed to verify checksum: %w", err)
			}

			if !valid {
				return fmt.Errorf("checksum verification failed")
			}

			log.Info("Checksum verification passed")
		} else {
			log.Info("Checksum file not found, skipping verification", "checksum_file", checksumPath)
		}
	}

	log.Info("File decrypted successfully",
		"input", inputFile,
		"key_file", keyFile,
		"output", outputFile)

	return nil
}
