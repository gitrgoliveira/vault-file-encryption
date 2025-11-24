# Vault File Encryption - AI Agent Instructions

## Project Overview

This Go application provides file encryption and decryption using HashiCorp Vault Transit Engine. It operates in two modes:
1. **CLI Mode**: For one-off encryption/decryption operations.
2. **Service Mode**: Watches directories for files and processes them continuously.

**Key Features**:
- Dual Vault support: HCP Vault (cloud) and Vault Enterprise (self-hosted).
- Envelope encryption with AES-256-GCM.
- Key re-wrapping for encryption key rotation.
- Offline key version auditing.
- Hot-reload configuration (Unix/Linux/macOS only).
- Pre-existing file scanning on startup.
- Comprehensive logging and audit support.

## Architecture Essentials

### Core Encryption Flow
1. Request plaintext DEK from Vault Transit.
2. Encrypt file locally with AES-256-GCM (chunked).
3. Vault encrypts DEK.
4. Save `.enc` (encrypted file) and `.key` (encrypted DEK).
5. Zero DEK from memory immediately with `SecureBuffer`.

### File Naming Convention
- Input: `example.txt`
- Outputs:
  - `example.txt.enc`: Encrypted file.
  - `example.txt.key`: Encrypted DEK (named after INPUT file, not .enc file).
  - `example.txt.sha256`: Optional checksum of plaintext.

### Service Mode Data Flow
1. **Startup**: Watcher scans directories for pre-existing files (both encrypt and decrypt operations).
2. **Runtime**: File system watcher detects new files via fsnotify events.
3. Stability check ensures file is fully written (waits for size to stabilize).
4. File is queued for processing (FIFO queue with persistence).
5. Processor encrypts/decrypts file using Vault.
6. Processed files are archived to visible subdirectories (`archive/`, `failed/`, `dlq/`).

### Race Condition Handling
When encryption creates `.enc` and `.key` files, fsnotify may fire CREATE event before `.key` exists:
- **Solution**: Watcher polls for `.key` file existence (100ms intervals, up to 1 second) before rejecting.
- **Location**: `internal/watcher/watcher.go:handleFileCreated()` lines ~200-220.
- This ensures newly encrypted files are immediately detected for decryption without restart.

## Development Workflows

### Build & Test
- **Build**: `make build` (creates `bin/file-encryptor`)
- **Build all platforms**: `make build-all` (Linux, macOS, Windows)
- **Test**: `make test` (unit tests with race detector and coverage)
- **Validate all**: `make validate-all` (runs fmt-check, vet, staticcheck, lint, gosec, and tests)
- **Integration Tests**: `make test-integration` (requires Vault setup)
- **Security scan**: `make gosec` (SAST with gosec)
- **Coverage report**: `make coverage` (generates coverage.html)

### Running the Application
- **CLI Mode**:
  ```bash
  ./bin/file-encryptor encrypt -i file.txt -o file.txt.enc -c configs/examples/example.hcl
  ./bin/file-encryptor decrypt -i file.txt.enc -k file.txt.key -o decrypted-file.txt -c configs/examples/example.hcl
  ```
- **Service Mode**:
  ```bash
  # Start watcher (scans pre-existing files on startup)
  ./bin/file-encryptor watch -c configs/examples/example.hcl
  
  # Hot-reload config (Unix/Linux/macOS only)
  pkill -SIGHUP file-encryptor
  ```
- **Key Re-wrapping**:
  ```bash
  # Re-wrap all .key files to minimum version 2
  ./bin/file-encryptor rewrap --dir /path/to/keys --recursive --min-version 2
  
  # Dry-run preview
  ./bin/file-encryptor rewrap --dir /path/to/keys --dry-run --min-version 2
  ```
- **Key Version Auditing** (offline, no Vault needed):
  ```bash
  ./bin/file-encryptor key-versions --dir /path/to/keys --recursive
  ```

### Development Setup
```bash
# Clone and setup
git clone https://github.com/gitrgoliveira/vault-file-encryption.git
cd vault-file-encryption
make deps

# Local Vault Enterprise dev mode
cd scripts/vault-setup-enterprise
./01-start-vault-dev.sh    # Terminal 1
./02-configure-vault.sh    # Terminal 2
vault agent -config=../../configs/vault-agent/vault-agent-enterprise-dev.hcl  # Terminal 3

# Run with dev config (relative paths from scripts/vault-setup-enterprise/)
cd scripts/vault-setup-enterprise
../../bin/file-encryptor watch -c ../../configs/examples/example-enterprise.hcl
```

## Project-Specific Conventions

### Code Style
- **File Naming**: Use lowercase with hyphens for directories, underscores for multi-word files.
- **Logging**: Always use structured logging with key-value pairs.
  ```go
  log.Info("processing file", "path", filepath, "size", fileSize)
  ```
- **Error Handling**: Wrap errors with context.
  ```go
  return fmt.Errorf("operation failed: %w", err)
  ```
- **Path Construction**: ALWAYS use `filepath.Join()` for cross-platform compatibility.
  ```go
  // Good
  archivePath := filepath.Join(cfg.SourceDir, "archive", filename)
  
  // Bad - breaks on Windows
  archivePath := cfg.SourceDir + "/.archive/" + filename
  ```

### Testing
- Use `t.TempDir()` for temporary files (auto-cleanup).
- Mock external dependencies (e.g., Vault client) using interfaces from `internal/interfaces/`.
- Test both small (<1MB) and large (>1MB) files for chunked operations.
- Use table-driven tests for multiple scenarios:
  ```go
  tests := []struct {
      name string
      input string
      want error
  }{
      {"valid", "test.txt", nil},
      {"invalid", "", ErrInvalidPath},
  }
  ```

### Security Patterns
- **SecureBuffer**: Automatic memory protection (locking + zeroing).
  ```go
  buf, _ := NewSecureBufferFromBytes(key)
  defer buf.Destroy()  // ALWAYS defer - zeroes memory on scope exit
  ```
- **Constant-time zeroing**: Use `crypto/subtle` to prevent compiler optimization.
- **Memory locking**: Keys locked in RAM via `mlock` (Unix/Linux/macOS only).
- Zero plaintext keys immediately after use with `defer SecureZero(key)`.

### Architecture Patterns
- **Strategy Pattern**: `internal/watcher/strategy.go` defines `ProcessStrategy` interface.
  - `EncryptStrategy` and `DecryptStrategy` implement file processing logic.
  - Processor selects strategy based on `model.OperationType`.
- **Separate FileHandlers**: Encryption and decryption use separate `FileHandler` instances.
  - `processor.FileHandler` for encryption operations.
  - `processor.decryptFileHandler` for decryption operations.
  - This allows different archive directories for encrypt vs decrypt.
- **Interface-based mocking**: All major components implement interfaces in `internal/interfaces/`.
  - `ConfigManager`, `Logger`, `VaultClient`, `Queue`, `Watcher`, `Processor`.
  - Tests use mocks implementing these interfaces.

## Key Files and Directories
- `cmd/file-encryptor/`: CLI entry point.
  - `main.go`: Command definitions (watch, encrypt, decrypt, rewrap, key-versions).
  - `rewrap.go`: Key re-wrapping implementation.
  - `key_versions.go`: Offline key version auditing (no Vault needed).
- `internal/crypto/`: Encryption/decryption logic.
  - `envelope.go`: Core encryption with chunked streaming.
  - `memory.go`, `memory_unix.go`, `memory_windows.go`: Platform-specific memory protection.
  - `secure_buffer.go`: Automatic key zeroing with defer pattern.
- `internal/watcher/`: File watching and processing.
  - `watcher.go`: fsnotify integration + pre-existing file scanning.
  - `processor.go`: File processing orchestration with separate handlers.
  - `strategy.go`: Strategy pattern for encrypt/decrypt operations.
  - `filehandler.go`: Post-processing (archive/delete/move).
- `internal/queue/`: FIFO queue with persistence and retry logic.
- `internal/service/`: Service lifecycle management.
- `internal/config/`: HCL configuration with hot-reload.
- `configs/examples/`: Configuration examples for HCP and Enterprise.
- `docs/`: Comprehensive documentation and guides.
  - `ARCHITECTURE.md`: Detailed system architecture.
  - `guides/CLI_MODE.md`: CLI usage (Unix/Linux/macOS).
  - `guides/REWRAP_GUIDE.md`: Key re-wrapping documentation.
  - `guides/CHUNK_SIZE_TUNING.md`: Performance optimization.

## Integration Points
- **Vault API**:
  - Generate DEK: `POST /v1/transit/datakey/plaintext/{key}`
  - Decrypt DEK: `POST /v1/transit/decrypt/{key}`
  - Rewrap DEK: `POST /v1/transit/rewrap/{key}`
- **File System**: Uses `fsnotify` for directory watching.

## Common Pitfalls
1. **NEVER** store plaintext DEKs on disk.
2. **ALWAYS** use `SecureBuffer` for sensitive data.
3. **NEVER** use `.enc.key` suffix; always use `.key` based on input filename.
4. **ALWAYS** validate chunk sizes during decryption.
5. **NEVER** skip `defer SecureZero()` after using plaintext keys.
6. **ALWAYS** use `filepath.Join()` for path construction (cross-platform).
7. **Race Condition**: When checking for `.key` file after detecting `.enc`, poll with retry (100ms intervals) instead of immediate rejection.
8. **Pre-existing Files**: `scanDirectory()` must be called on startup for both encryption and decryption sources.
9. **Separate FileHandlers**: Use different handlers for encryption and decryption to support different archive directories.
10. **Visible Subdirectories**: Use `archive/`, `failed/`, `dlq/` (not hidden `.archive/`, etc.).

## Additional Resources
- [Architecture Guide](../docs/ARCHITECTURE.md)
- [CLI Guide](../docs/guides/CLI_MODE.md)
- [Rewrap Guide](../docs/guides/REWRAP_GUIDE.md)
- [Chunk Size Tuning](../docs/guides/CHUNK_SIZE_TUNING.md)
