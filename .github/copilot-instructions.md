# Vault File Encryption - AI Agent Instructions

Never use unicode or special characters in code or documentation

## Project Overview

Production-ready Go application that encrypts/decrypts files using HashiCorp Vault Transit Engine with envelope encryption. Operates in two modes: (1) service mode for continuous directory watching (Phase 4), and (2) CLI mode for one-off operations (Phase 3 - COMPLETE).

**Current Status**: Phase 5 COMPLETE (Nov 2025) - Full HCL configuration management with hot-reload, audit logging, and enhanced logging. Always check IMPLEMENTATION_PLAN.md and PHASE_*.md before making changes.

## Architecture Essentials

### Operating Modes
- **CLI mode** (COMPLETE - Phase 3): Direct encrypt/decrypt commands with progress logging
- **Service mode** (COMPLETE - Phase 4): Continuous file watching with FIFO queue processing and exponential backoff retry
- **Configuration** (COMPLETE - Phase 5): Full HCL parsing, hot-reload via SIGHUP, audit logging

### Envelope Encryption Implementation (Phase 3 Complete)
1. Vault generates plaintext data key (DEK) via Transit Engine
2. File encrypted locally with AES-256-GCM using DEK (chunked: 1MB per chunk)
3. DEK encrypted by Vault, saved as `.key` file
4. Plaintext DEK zeroed from memory with `defer SecureZero()`
5. Output files: `file.enc` (encrypted data) + `file.key` (encrypted DEK) + `file.sha256` (optional)

**File Format**:
- `.enc`: [12-byte nonce][chunk1_size(4 bytes)][chunk1_ciphertext]...[chunkN_size][chunkN_ciphertext]
- `.key`: `vault:v1:base64encodedencryptedDEK`
- `.sha256`: `<hex-sha256-hash>`

**Vault Access**: 
- **Development**: Direct HTTPS to HCP Vault with environment variables (VAULT_ADDR, VAULT_TOKEN, VAULT_NAMESPACE) - acceptable for Phase 3-5
- **Production** (Phase 6+): Via Vault Agent listener at http://127.0.0.1:8200 with certificate auth

**HCP Vault Configuration** (Phase 2 deployed via Terraform):
- **Cluster**: `vault-cluster-primary.vault.11eab575-aee3-cf27-adc9-0242ac11000a.aws.hashicorp.cloud:8200`
- **Namespace**: `admin/vault_crypto`
- **Transit Mount**: `transit/`
- **Key Name**: `file-encryption-key` (AES-256-GCM)
- **Policy**: `file-encryptor-policy` (datakey generate/decrypt only, denies direct encrypt)

### Critical Data Flow (Service Mode)
```
fsnotify → stability check (1s) → queue enqueue → processor dequeue
→ request DEK from Vault → encrypt with AES-GCM → save .enc/.key
→ zero DEK → archive/delete source → log completion
```

## Development Workflows

### Vault Setup (Phase 2 - Already Deployed)
```bash
# Vault infrastructure is deployed via Terraform
# To verify/modify:
cd scripts/vault-setup
export VAULT_ADDR="https://vault-cluster-primary.vault.11eab575-aee3-cf27-adc9-0242ac11000a.aws.hashicorp.cloud:8200"
export VAULT_TOKEN="<from .env file>"
export VAULT_NAMESPACE="admin/vault_crypto"

terraform plan              # Review changes
terraform apply             # Deploy changes
terraform output            # View deployed resources

# Test data key operations:
vault write -f transit/datakey/plaintext/file-encryption-key  # Generate DEK
vault read transit/keys/file-encryption-key                   # Check key status
```

**Terraform State Management**:
- State stored locally: `scripts/vault-setup/terraform.tfstate`
- Backup: `terraform.tfstate.backup` (auto-created)
- Both excluded from git via `.gitignore`
- **Never commit** state files (may contain sensitive data)
- To inspect state: `terraform show` or `terraform state list`
- To modify deployed resources: Edit `.tf` files, then `terraform apply`

**What's Deployed**:
1. Transit Engine at `transit/` with AES-256-GCM key
2. Certificate auth backend at `auth/cert/` with role `file-encryptor`
3. Policy `file-encryptor-policy` (datakey generate/decrypt only)
4. Test certificates in `scripts/test-certs/` (4096-bit RSA, 10-year validity)

### Vault Agent Transition (Development → Production)

**Phase 3-5 Development** (Current):
- Use direct HTTPS to HCP Vault with token from `.env`
- Simpler: No agent process, no mTLS setup
- Pattern in code:
  ```go
  client := &http.Client{}
  req.Header.Set("X-Vault-Token", os.Getenv("VAULT_TOKEN"))
  req.Header.Set("X-Vault-Namespace", os.Getenv("VAULT_NAMESPACE"))
  ```

**Phase 6+ Production**:
- Deploy Vault Agent as sidecar/daemon process
- Agent config: `configs/vault-agent.hcl` (template provided)
- Certificate-based auto-authentication (mTLS)
- Application connects to `http://127.0.0.1:8200` (agent listener)
- Pattern in code:
  ```go
  client := &http.Client{}
  req, _ := http.NewRequest("POST", "http://127.0.0.1:8200/v1/transit/datakey/plaintext/file-encryption-key", nil)
  // No X-Vault-Token needed - Agent handles authentication
  ```

**When to Transition**:
- Development/Testing: Direct token access is fine (through Phase 5)
- Staging/Production: **Must** use Vault Agent for:
  - Automatic token renewal
  - Certificate rotation
  - Request caching (reduces Vault load)
  - Token leakage prevention (no tokens in app config)

**Agent Startup** (production):
```bash
vault agent -config=/etc/vault-agent/vault-agent.hcl
# Or as systemd service, Docker sidecar, or Kubernetes init container
```

### Build & Run
```bash
make build              # Build for current platform
make build-all          # Cross-compile for Linux/macOS/Windows

# CLI Mode (Phase 3 - COMPLETE)
export VAULT_ADDR="https://vault-cluster-primary.vault.11eab575-aee3-cf27-adc9-0242ac11000a.aws.hashicorp.cloud:8200"
export VAULT_TOKEN="<from .env file>"
export VAULT_NAMESPACE="admin/vault_crypto"

./bin/file-encryptor encrypt -i file.txt -o file.enc --checksum -c configs/dev-config.hcl
./bin/file-encryptor decrypt -i file.enc -k file.enc.key -o file-decrypted.txt -c configs/dev-config.hcl

# Service Mode (Phase 4 - COMPLETE)
./bin/file-encryptor watch -c configs/dev-config.hcl

# Hot-reload configuration (Phase 5 - COMPLETE)
kill -HUP <pid>  # Or pkill -SIGHUP file-encryptor
```

### Testing
```bash
make test               # Unit tests (100+ tests, all passing)
make test-integration   # Integration tests (coming in Phase 6)
make coverage           # Coverage report
```

**Test Pattern**: Tests live in `internal/*/` alongside code files (*_test.go). Use table-driven tests with `testify/assert` and `testify/require`. Integration tests will go in `test/integration/`.

**Current Coverage** (Phase 5):
- Config: 32 tests (parsing, validation, hot-reload)
- Logger: 18 tests (levels, outputs, audit logging)
- Crypto: 11 tests (encryption, decryption, checksums)
- Queue: 16 tests (FIFO, retries, persistence)
- Vault: 9 tests (client, data keys)
- Watcher: 4 tests (stability detection)

### Version Management
Version info injected via ldflags in Makefile. Update via:
- Git tags for releases: `git tag v0.2.0`
- Version displays from `internal/version/version.go`
- Build time and commit hash auto-populated

### Environment Variables
Development uses `.env` file (excluded from git):
```bash
VAULT_ADDR=https://vault-cluster-primary.vault.11eab575-aee3-cf27-adc9-0242ac11000a.aws.hashicorp.cloud:8200
VAULT_TOKEN=<admin-token>
VAULT_NAMESPACE=admin/vault_crypto
```
Application reads these for direct Vault access during Phase 3 development.

## Code Conventions

### Package Structure
- `cmd/file-encryptor/main.go`: CLI entry point, Cobra commands, signal handling
- `internal/*/`: Private packages not importable externally
  - `config`: HCL parsing with hot-reload (Phase 5 - COMPLETE)
  - `crypto`: Encryption/decryption (Phase 3 - COMPLETE)
  - `vault`: Vault client (Phase 3 - COMPLETE)
  - `watcher`: File watching (Phase 4 - COMPLETE)
  - `queue`: FIFO queue with persistence (Phase 4 - COMPLETE)
  - `logger`: Structured logging with audit support (Phase 5 - COMPLETE)
  - `version`: Build metadata

### Logging Pattern
```go
log.Info("message", "key", value, "key2", value2)  // key-value pairs
log.Error("error message", "error", err)
log.Debug("debug info")  // Only logs if level=debug
```
Always use structured logging with key-value pairs, never string concatenation.

### Error Handling
- Wrap errors with context: `fmt.Errorf("operation failed: %w", err)`
- Return errors to caller, don't log and return
- Use `defer` for cleanup (e.g., `defer log.Sync()`)
- Zero sensitive data with `SecureZero()` in defer blocks

### Configuration
- Primary format: HCL (see `configs/example.hcl`)
- Hot-reload on SIGHUP via `config.Manager` (Phase 5 - COMPLETE)
- Validation in `config.Validate()` before use
- Duration support: Use strings like "30s", "5m", "1h" in HCL files
- Thread-safe access via `ConfigManager.GetConfig()` and `UpdateConfig()`

## Project-Specific Patterns

### Signal Handling (main.go)
- **SIGTERM/SIGINT**: Immediate shutdown, cancel context, save queue state
- **SIGHUP**: Hot-reload configuration via `ConfigManager.Reload()` (Phase 5 - COMPLETE)
- Don't wait for current file to finish processing on shutdown

### File Processing States
- **Queue Item States**: pending → processing → completed/failed/dlq
- **Source File Behaviors**: "archive" (default), "delete", or "keep"
- **Retry Logic**: Exponential backoff with max retries (configurable)

### Progress Logging
Log file processing progress at 20% intervals when encrypting/decrypting large files. Example:
```go
// Log when progress >= next milestone (20%, 40%, 60%, 80%, 100%)
if progress >= nextMilestone {
    log.Info("Encryption progress", "file", filename, "progress", progress)
}
```

### Partial Upload Detection
Files must be stable (no size change) for 1 second before processing. Prevents encrypting incomplete uploads.

## Integration Points

### Vault Transit Engine
- **Endpoints**: `/v1/transit/datakey/plaintext/{key}` (generate), `/v1/transit/decrypt/{key}` (decrypt)
- **Development**: Direct HTTPS to HCP Vault using token from .env (Phase 3)
- **Production**: Via Vault Agent at `http://127.0.0.1:8200` (Phase 5+)
- **Mount path**: Configurable, default "transit"
- **Key name**: Configurable, default "file-encryption-key"

### File System Watching
- Use `fsnotify` package (Phase 4 - IMPLEMENTED)
- Watch `source_dir` recursively
- Filter by `file_pattern` glob if configured
- Check file stability before queuing

### Queue Persistence
- State file: JSON format at `queue.state_path` (default: queue-state.json)
- Atomic writes: Write to temp file, then rename
- Load on startup, save on shutdown and periodically
- Includes retry counts and next_retry timestamps

## Common Pitfalls

1. **DON'T** call Vault directly in production - use Vault Agent listener (dev: direct OK with token)
2. **DON'T** store plaintext DEKs on disk - only in memory, zeroed after use with `defer SecureZero()`
3. **DON'T** implement features ahead of phases - follow IMPLEMENTATION_PLAN.md
4. **DON'T** use `log.Fatal()` in library code - return errors instead
5. **DON'T** forget to increment nonce between chunks in chunked encryption
6. **DO** check phase documents (PHASE_*.md) before implementing features
7. **DO** use structured logging with key-value pairs
8. **DO** validate configuration before using it
9. **DO** test with both small files (<1MB) and large files (>1MB) to verify chunking
10. **DO** use progress callbacks for large file operations to provide user feedback

## Key Files to Reference

- `ARCHITECTURE.md`: Complete system design, data flows, security model
- `IMPLEMENTATION_PLAN.md`: Phase breakdown, timeline, dependencies
- `PHASE_*.md`: Detailed implementation guides for each component
- `PHASE_3_COMPLETION.md`: Phase 3 implementation summary and test results
- `PHASE_4_COMPLETION.md`: Phase 4 implementation summary and test results
- `PHASE_5_COMPLETION.md`: Phase 5 implementation summary and test results
- `configs/example.hcl`: Complete configuration example with comments
- `cmd/file-encryptor/main.go`: CLI structure, signal handling patterns, encrypt/decrypt implementations
- `internal/config/config.go`: Configuration structs with HCL tags
- `internal/config/loader.go`: HCL file parsing
- `internal/config/validator.go`: Configuration validation with directory auto-creation
- `internal/config/reload.go`: Hot-reload manager with callbacks
- `internal/logger/logger.go`: Logging pattern reference with levels
- `internal/logger/audit.go`: JSON-based audit logging
- `internal/vault/client.go`: Vault client implementation patterns
- `internal/crypto/envelope.go`: Chunked encryption/decryption implementation
- `internal/queue/queue.go`: Thread-safe FIFO queue with exponential backoff
- `internal/watcher/watcher.go`: File system monitoring with fsnotify

## Current Implementation Status

**Phase 1 Complete**: Project structure, basic CLI, dependencies, Makefile
**Phase 2 Complete**: Vault setup, Terraform deployment, test certificates, policies
**Phase 3 Complete**: Vault client, envelope encryption/decryption, CLI integration, unit tests
**Phase 4 Complete**: Watcher/queue (service mode), file stability detection, retry logic, state persistence
**Phase 5 Complete**: Config management (HCL parsing, hot-reload, enhanced logging, audit support)
**Phase 6 Pending**: Testing/CI/CD (integration tests, GitHub Actions, deployment)

When implementing new features:
1. Check IMPLEMENTATION_PLAN.md for phase dependencies
2. Review corresponding PHASE_*.md document
3. Follow patterns in existing code (logger, config, version, vault, crypto)
4. Add TODO comments for cross-phase dependencies
5. Update tests in `test/` directory
6. Use `defer SecureZero()` for all sensitive data (DEKs, plaintexts)
7. Provide progress callbacks for operations that may take >1 second

## Module and Imports

Module name: `github.com/gitrgoliveira/vault_file_encryption`

Standard import pattern:
```go
import (
    "context"
    "fmt"
    
    "github.com/gitrgoliveira/vault_file_encryption/internal/config"
    "github.com/gitrgoliveira/vault_file_encryption/internal/logger"
)
```
```
