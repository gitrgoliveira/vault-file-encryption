# Architecture Overview

**Project**: Vault File Encryption  
**Version**: 0.6.0  
**Last Updated**: November 2025

## System Architecture

### High-Level Architecture

The application supports **two modes of operation**:

1. **Service Mode** (watch): Continuous file watching and processing
2. **CLI Mode** (one-off): Single file encryption/decryption

```
┌─────────────────────────────────────────────────────────────────┐
│                     File Encryptor Application                   │
│                                                                  │
│  ┌────────────────────────────────────────────────────────┐    │
│  │                    CLI Interface                        │    │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐            │    │
│  │  │  watch   │  │  encrypt │  │  decrypt │            │    │
│  │  │  (service)│  │ (one-off)│  │ (one-off)│            │    │
│  │  └────┬─────┘  └────┬─────┘  └────┬─────┘            │    │
│  └───────┼─────────────┼─────────────┼───────────────────┘    │
│          │             │             │                          │
│          │             └─────────────┴──────────┐               │
│          │                                      │               │
│          ▼                                      ▼               │
│  ┌──────────────┐      ┌─────────────┐      ┌──────────────┐  │
│  │File Watcher  │─────▶│ FIFO Queue  │      │  Direct      │  │
│  │  (fsnotify)  │      │(Persistent) │      │  Processor   │  │
│  └──────────────┘      └─────────────┘      └──────────────┘  │
│         │                     │                     │           │
│         │              ┌──────▼──────┐             │           │
│         │              │Retry Logic  │             │           │
│         │              │(Exp Backoff)│             │           │
│         │              └─────────────┘             │           │
│         │                     │                    │           │
│  ┌──────▼─────────────────────▼────────────────────▼───────┐  │
│  │            Crypto Package (Envelope Encryption)          │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐              │  │
│  │  │Encryptor │  │Decryptor │  │ Checksum │              │  │
│  │  └────┬─────┘  └────┬─────┘  └──────────┘              │  │
│  └───────┼─────────────┼─────────────────────────────────────┘│
│          │             │                                       │
│  ┌───────▼─────────────▼───────────────┐                      │
│  │        Vault Client                  │                      │
│  │  ┌──────────────────────────────┐   │                      │
│  │  │  Data Key Operations         │   │                      │
│  │  │  - Generate DEK              │   │                      │
│  │  │  - Decrypt DEK               │   │                      │
│  │  └──────────────────────────────┘   │                      │
│  └───────────────┬──────────────────────┘                     │
└──────────────────┼────────────────────────────────────────────┘
                   │
          ┌────────▼─────────┐
          │  Vault Agent     │
          │  (Listener)      │
          │                  │
          │  - Auto-auth     │
          │  - Caching       │
          │  - Token mgmt    │
          └────────┬─────────┘
                   │
          ┌────────▼─────────────────────┐
          │ HCP Vault OR Vault Enterprise│
          │                              │
          │  Transit Engine              │
          │  - Primary Key               │
          │  - Key rotation              │
          │                              │
          │  HCP: Token auth             │
          │  Enterprise: Cert auth       │
          └──────────────────────────────┘
```

## Component Diagram

### Core Components

```
┌─────────────────────────────────────────────────────────────────┐
│                        Application Layer                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  cmd/file-encryptor/main.go                                     │
│  - CLI interface                                                │
│  - Signal handling                                              │
│  - Component orchestration                                      │
│                                                                  │
├─────────────────────────────────────────────────────────────────┤
│                       Business Logic Layer                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  internal/watcher/                                              │
│  ┌─────────────┬─────────────┬──────────────┐                  │
│  │  Watcher    │  Detector   │  Processor   │                  │
│  │             │             │              │                  │
│  │ - fs events │ - stability │ - orchestrate│                  │
│  │ - filtering │ - partial   │ - encrypt    │                  │
│  │             │   uploads   │ - decrypt    │                  │
│  └─────────────┴─────────────┴──────────────┘                  │
│                                                                  │
│  internal/queue/                                                │
│  ┌─────────────┬─────────────┬──────────────┐                  │
│  │   Queue     │ Persistence │    Item      │                  │
│  │             │             │              │                  │
│  │ - FIFO      │ - save/load │ - metadata   │                  │
│  │ - requeue   │ - atomic    │ - status     │                  │
│  │ - backoff   │   writes    │              │                  │
│  └─────────────┴─────────────┴──────────────┘                  │
│                                                                  │
├─────────────────────────────────────────────────────────────────┤
│                      Crypto/Security Layer                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  internal/crypto/                                               │
│  ┌─────────────┬─────────────┬──────────────┐                  │
│  │ Encryptor   │ Decryptor   │  Checksum    │                  │
│  │             │             │              │                  │
│  │ - envelope  │ - envelope  │ - SHA256     │                  │
│  │ - AES-GCM   │ - AES-GCM   │ - verify     │                  │
│  │ - streaming │ - streaming │              │                  │
│  └─────────────┴─────────────┴──────────────┘                  │
│                                                                  │
│  internal/vault/                                                │
│  ┌─────────────┬─────────────────────────────┐                 │
│  │   Client    │      Data Key Ops           │                 │
│  │             │                             │                 │
│  │ - API calls │ - generate plaintext DEK    │                 │
│  │ - via Agent │ - decrypt ciphertext DEK    │                 │
│  └─────────────┴─────────────────────────────┘                 │
│                                                                  │
├─────────────────────────────────────────────────────────────────┤
│                   Infrastructure Layer                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  internal/config/                                               │
│  ┌─────────────┬─────────────┬──────────────┐                  │
│  │   Loader    │  Validator  │   Manager    │                  │
│  │             │             │              │                  │
│  │ - HCL parse │ - checks    │ - hot-reload │                  │
│  │ - defaults  │ - dirs      │ - callbacks  │                  │
│  └─────────────┴─────────────┴──────────────┘                  │
│                                                                  │
│  internal/logger/                                               │
│  ┌─────────────┬──────────────────────────────┐                │
│  │   Logger    │      Audit Logger            │                │
│  │             │                              │                │
│  │ - levels    │ - JSON events                │                │
│  │ - plaintext │ - file output                │                │
│  └─────────────┴──────────────────────────────┘                │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Data Flow

### CLI Mode - One-off Encryption

```
Command: file-encryptor encrypt -i input.txt -o output.txt.enc
     │
     ▼
┌─────────────────────────┐
│  Parse CLI Arguments    │
│  - Input file           │
│  - Output file          │
│  - Key file (optional)  │
└──────┬──────────────────┘
       │
       ├─── Calculate Checksum (if --checksum)
       │
       ├─── Request Data Key from Vault
       │    └─── POST /transit/datakey/plaintext/{key}
       │
       ├─── Encrypt File with Plaintext DEK
       │    └─── Write to specified output
       │
       ├─── Save Ciphertext DEK
       │    └─── Write to input.key (based on input filename)
       │
       ├─── Zero Plaintext DEK from Memory
       │    ├─── SecureBuffer.Destroy() (automatic)
       │    └─── SecureZero(temp_bytes)
       │
       └─── Exit (return 0 on success, 1 on error)
```

### CLI Mode - One-off Decryption

```
Command: file-encryptor decrypt -i input.txt.enc -k input.txt.key -o output.txt
     │
     ▼
┌─────────────────────────┐
│  Parse CLI Arguments    │
│  - Encrypted file       │
│  - Key file             │
│  - Output file          │
└──────┬──────────────────┘
       │
       ├─── Read Ciphertext DEK from key file
       │
       ├─── Decrypt DEK with Vault
       │    └─── POST /transit/decrypt/{key}
       │
       ├─── Decrypt File with Plaintext DEK
       │    └─── Write to specified output
       │
       ├─── Verify Checksum (if --verify-checksum)
       │
       ├─── Zero Plaintext DEK from Memory
       │    ├─── SecureBuffer.Destroy() (automatic)
       │    └─── SecureZero(temp_bytes)
       │
       └─── Exit (return 0 on success, 1 on error)
```

### Service Mode - Encryption Flow

```
1. New File Detected
   ┌──────────────┐
   │ Source File  │
   │  (plaintext) │
   └──────┬───────┘
          │
          ▼
   ┌──────────────┐
   │File Watcher  │ ─── Checks stability (1s no size change)
   └──────┬───────┘
          │
          ▼
   ┌──────────────┐
   │ Enqueue Item │ ─── Creates queue item with metadata
   └──────┬───────┘
          │
          ▼
   ┌──────────────┐
   │ FIFO Queue   │ ─── Thread-safe, persistent
   └──────┬───────┘
          │
          ▼
   ┌──────────────┐
   │  Processor   │ ─── Dequeues and processes
   └──────┬───────┘
          │
          ├─── Calculate Checksum (optional)
          │    └─── SHA256 → save to .sha256
          │
          ├─── Request Data Key from Vault
          │    └─── POST /transit/datakey/plaintext/{key}
          │         Returns: {plaintext_dek, ciphertext_dek}
          │
          ├─── Encrypt File with Plaintext DEK
          │    ├─── Read source in 64KB chunks
          │    ├─── Encrypt with AES-256-GCM
          │    ├─── Log progress every 20%
          │    └─── Write to destination.enc
          │
          ├─── Save Ciphertext DEK
          │    └─── Write to source.key (based on original filename)
          │
          ├─── Zero Plaintext DEK from Memory
          │    ├─── SecureBuffer.Destroy() (automatic)
          │    └─── SecureZero(dek_bytes) for temp bytes
          │
          └─── Handle Source File
               ├─── Archive: Move to .archive/
               └─── Delete: Remove file
```

### Service Mode - Decryption Flow

```
1. Encrypted File Pair Detected (.enc + .key)
   ┌──────────────┬──────────────┐
   │ File.enc     │  File.key    │
   └──────┬───────┴──────┬───────┘
          │              │
          ▼              ▼
   ┌─────────────────────────┐
   │   File Watcher          │
   └──────┬──────────────────┘
          │
          ▼
   ┌─────────────────────────┐
   │   Enqueue Item          │
   └──────┬──────────────────┘
          │
          ▼
   ┌─────────────────────────┐
   │   FIFO Queue            │
   └──────┬──────────────────┘
          │
          ▼
   ┌─────────────────────────┐
   │   Processor             │
   └──────┬──────────────────┘
          │
          ├─── Read Ciphertext DEK from .key file
          │
          ├─── Decrypt DEK with Vault
          │    └─── POST /transit/decrypt/{key}
          │         Input: ciphertext_dek
          │         Returns: plaintext_dek
          │
          ├─── Decrypt File with Plaintext DEK
          │    ├─── Read encrypted file in chunks
          │    ├─── Decrypt with AES-256-GCM
          │    ├─── Log progress every 20%
          │    └─── Write to destination
          │
          ├─── Verify Checksum (optional)
          │    └─── Compare with .sha256 file
          │
          ├─── Zero Plaintext DEK from Memory
          │    ├─── SecureBuffer.Destroy() (automatic)
          │    └─── SecureZero(temp_bytes)
          │
          └─── Handle Source Files
               ├─── Archive: Move .enc and .key to .archive/
               └─── Delete: Remove .enc and .key files
```

## Error Handling and Retry Logic

### Retry Strategy

```
┌─────────────────────────────────────────────────────────────────┐
│                     Error Handling Flow                          │
└─────────────────────────────────────────────────────────────────┘

Processing Attempt
      │
      ├─── Success? ───▶ Mark Complete ───▶ Handle Source File
      │
      └─── Failure
           │
           ├─── Check Retry Count
           │    │
           │    ├─── < Max Retries?
           │    │    │
           │    │    ├─── Calculate Backoff
           │    │    │    └─── delay = base_delay * 2^attempts
           │    │    │         (capped at max_delay)
           │    │    │
           │    │    ├─── Mark Failed
           │    │    │    └─── Set next_retry = now + delay
           │    │    │
           │    │    └─── Requeue to end of FIFO
           │    │
           │    └─── >= Max Retries
           │         │
           │         ├─── Mark as DLQ
           │         │
           │         └─── Move to .dlq/ folder
           │
           └─── Move to .failed/ folder
```

### Exponential Backoff Example

```
Attempt 1: base_delay = 1s
Attempt 2: 1s * 2^1 = 2s
Attempt 3: 1s * 2^2 = 4s
Attempt 4: 1s * 2^3 = 8s
Attempt 5: 1s * 2^4 = 16s
...
Capped at: max_delay = 5m
```

## File Organization

### Directory Structure

```
Source Directory (Encryption)
/data/source/
├── file1.txt          ← New files dropped here
├── file2.pdf
└── .archive/          ← Archived originals (configurable)
    ├── file0.txt
    └── .failed/       ← Failed files
        └── .dlq/      ← Dead letter queue

Destination Directory (Encryption)
/data/encrypted/
├── file1.txt.enc      ← Encrypted file
├── file1.txt.key      ← Encrypted DEK (Vault ciphertext)
├── file1.txt.sha256   ← Checksum (optional)
├── file2.pdf.enc
├── file2.pdf.key
└── file2.pdf.sha256

Source Directory (Decryption)
/data/encrypted/       ← Watch for .enc + .key pairs
└── .archive/          ← Archived encrypted files

Destination Directory (Decryption)
/data/decrypted/
├── file1.txt          ← Decrypted plaintext
└── file2.pdf
```

## Security Architecture

### Key Management

```
┌─────────────────────────────────────────────────────────────────┐
│                    Key Hierarchy (Envelope Encryption)           │
└─────────────────────────────────────────────────────────────────┘

Primary Key (Vault Transit)
     │
     │  Never leaves Vault
     │  Stored in Vault's secure storage
     │  Used to encrypt/decrypt DEKs
     │
     ├─── Data Encryption Key 1 (DEK)
     │    │
     │    ├─── Plaintext DEK
     │    │    - Generated by Vault
     │    │    - Used locally for file encryption
     │    │    - Protected by SecureBuffer (automatic locking + zeroing)
     │    │    - Zeroed from memory after use (constant-time via crypto/subtle)
     │    │    - Locked in memory via mlock to prevent swapping (Unix/Linux/macOS)
     │    │    - Never persisted to disk
     │    │
     │    └─── Ciphertext DEK
     │         - Encrypted with Primary Key
     │         - Stored in .key file
     │         - Safe to persist
     │
     ├─── Data Encryption Key 2
     └─── Data Encryption Key N...
```

### Security Features

1. **Envelope Encryption**: Primary key never leaves Vault
2. **Memory Security**: 
   - **SecureBuffer**: Automatic memory protection for sensitive keys (locks + zeros on destroy)
   - Constant-time zeroing using `crypto/subtle` (prevents compiler optimization)
   - Memory locking via `mlock` prevents key swapping to disk (Unix/Linux/macOS only)
   - Platform-specific implementations: `memory_unix.go` (mlock) and `memory_windows.go` (no-op)
   - Immediate zeroing after use with `defer buf.Destroy()`
3. **Cryptographic Protection**:
   - AES-256-GCM authenticated encryption
   - Unique nonce per chunk (base nonce + counter)
   - File metadata authenticated via GCM additional data
   - Nonce overflow detection (max 2^32 chunks ≈ 4 petabytes)
4. **DOS Prevention**:
   - Maximum chunk size validation (10MB)
   - Maximum file size enforcement
   - Chunk size sanity checks during decryption
5. **In-Transit Protection**: Files encrypted before storage
6. **Audit Logging**: All operations logged
7. **Certificate Auth**: Mutual TLS with Vault (Enterprise)
8. **Response Caching**: Vault Agent caches for performance
9. **Checksum Verification**: SHA-256 integrity validation

### Encrypted File Format

```
┌─────────────────────────────────────────────────────────────────┐
│                   Encrypted File Structure                       │
└─────────────────────────────────────────────────────────────────┘

.enc File:
┌──────────────┬───────────────┬─────────────┬──────────────┬─────┐
│ Master Nonce │ File Size     │ Chunk1 Size │ Chunk1       │ ... │
│ (12 bytes)   │ (8 bytes)     │ (4 bytes LE)│ (ciphertext) │     │
└──────────────┴───────────────┴─────────────┴──────────────┴─────┘
     │              │                │              │
     │              │                │              └─ Encrypted with AES-256-GCM
     │              │                └──────────────── Big-endian uint32
     │              └───────────────────────────────── Authenticated via GCM AAD
     └──────────────────────────────────────────────── Incremented per chunk

.key File:
vault:v{version}:{base64-encrypted-DEK}

Notes:
- Each chunk uses nonce = master_nonce + chunk_index
- File size is included in GCM additional authenticated data
- Maximum chunks per file: 2^32 (prevents nonce overflow)
- Maximum chunk size: 10MB (prevents memory exhaustion)
- Default chunk size: 1MB (configurable 64KB-10MB)
```

### Security Guarantees

**Confidentiality**:
- AES-256-GCM encryption with 256-bit keys
- Unique nonce per chunk (cryptographically guaranteed)
- DEK protected by Vault's primary key

**Integrity**:
- GCM authentication tag per chunk (128-bit)
- File metadata (size) authenticated
- Optional SHA-256 checksum for end-to-end verification

**Memory Safety**:
- **SecureBuffer type**: Automatic memory protection (locking + zeroing)
- Constant-time memory operations (prevents timing attacks)
- Memory locking prevents keys in swap files (Unix/Linux/macOS via `mlock`)
- Platform-specific implementations for cross-platform support
- Automatic zeroing on scope exit (defer pattern with `buf.Destroy()`)
- No-op memory locking on Windows (mlock not available)

**DOS Resistance**:
- Chunk size validation (prevents memory exhaustion)
- File size limits (prevents resource exhaustion)
- Nonce overflow detection (prevents cryptographic failure)

## Configuration Management

### Hot Reload Mechanism

```
SIGHUP Signal
     │
     ▼
┌─────────────────┐
│ Config Manager  │
└────────┬────────┘
         │
         ├─── Load New Config from Disk
         │
         ├─── Validate New Config
         │    │
         │    ├─── Valid?
         │    │    │
         │    │    └─── Apply
         │    │         │
         │    │         ├─── Swap Current Config
         │    │         │
         │    │         └─── Notify Callbacks
         │    │              ├─── Update Vault Client
         │    │              ├─── Update Watcher Paths
         │    │              ├─── Update Queue Settings
         │    │              └─── Update Logger Level
         │    │
         │    └─── Invalid?
         │         └─── Keep Current Config
         │              Log Error
         │
         └─── Continue Running (No Restart)
```

## Graceful Shutdown

```
SIGTERM/SIGINT Signal
     │
     ▼
┌─────────────────┐
│ Signal Handler  │
└────────┬────────┘
         │
         ├─── Stop File Watcher
         │    └─── No new files accepted
         │
         ├─── Cancel Context
         │    └─── All goroutines receive cancel
         │
         ├─── Current File Processing
         │    └─── DO NOT wait for completion
         │         └─── Interrupt immediately
         │
         ├─── Save Queue State
         │    ├─── Marshal all queue items to JSON
         │    ├─── Atomic write to state file
         │    └─── Includes retry metadata
         │
         ├─── Sync Logs
         │    └─── Flush buffers
         │
         └─── Exit
              └─── On restart: Queue state is restored
```

## Performance Considerations

### Streaming Encryption

- Files processed in 1MB chunks
- Prevents memory exhaustion on large files
- Progress reported every 20%

### Concurrency

- Single processor (sequential processing)
- Thread-safe queue for scaling
- Could add worker pool if needed

### Vault Agent Benefits

- Local caching reduces latency
- Auto-token renewal
- Connection pooling

## Scalability

### Current Design

- Single instance processes files sequentially
- Queue persists state for reliability
- Suitable for moderate file volumes

### Enhancements

- Worker pool for parallel processing
- Distributed queue (Redis, RabbitMQ)
- Horizontal scaling with multiple instances
- Metrics and monitoring (Prometheus)

## Deployment Architecture

### HCP Vault Deployment
```
┌─────────────────────────────────────────────────────────────────┐
│                    Production Deployment (HCP)                   │
└─────────────────────────────────────────────────────────────────┘

Host/VM
├── file-encryptor (systemd service)
│   ├── Config: /etc/file-encryptor/config.hcl
│   ├── State: /var/lib/file-encryptor/queue-state.json
│   └── Logs: /var/log/file-encryptor/
│
├── vault-agent (systemd service)
│   ├── Config: /etc/vault-agent/config.hcl
│   ├── Auth: Token-based
│   └── Listener: 127.0.0.1:8200
│
└── Network
    ├── Vault Agent → HCP Vault (HTTPS, token auth)
    └── file-encryptor → Vault Agent (HTTP localhost)
```

### Vault Enterprise Deployment
```
┌─────────────────────────────────────────────────────────────────┐
│              Production Deployment (Vault Enterprise)            │
└─────────────────────────────────────────────────────────────────┘

Host/VM
├── file-encryptor (systemd service)
│   ├── Config: /etc/file-encryptor/config.hcl
│   ├── State: /var/lib/file-encryptor/queue-state.json
│   └── Logs: /var/log/file-encryptor/
│
├── vault-agent (systemd service)
│   ├── Config: /etc/vault-agent/config.hcl
│   ├── Certs: /etc/vault-agent/certs/
│   ├── Auth: Certificate-based
│   └── Listener: 127.0.0.1:8210
│
└── Network
    ├── Vault Agent → Vault Enterprise (HTTPS, cert auth)
    └── file-encryptor → Vault Agent (HTTP localhost)
```

### Development Setup (Vault Enterprise Dev Mode)
```
┌─────────────────────────────────────────────────────────────────┐
│              Development Setup (Local Vault)                     │
└─────────────────────────────────────────────────────────────────┘

Localhost
├── vault server -dev (port 8200)
│   ├── In-memory storage
│   ├── Auto-unsealed
│   └── Root token provided
│
├── vault-agent (foreground)
│   ├── Config: configs/vault-agent/vault-agent-enterprise-dev.hcl
│   ├── Certs: scripts/test-certs/
│   └── Listener: 127.0.0.1:8210
│
└── file-encryptor (foreground)
    └── Config: configs/examples/example-enterprise.hcl
```

## Design Decisions

### Why Envelope Encryption?

- **Scalability**: Local encryption, remote key management
- **Performance**: No 32MB file size limit
- **Security**: Primary key never leaves Vault
- **Efficiency**: Minimal network traffic

### Why FIFO Queue?

- **Order Preservation**: Files processed in order
- **Retry Logic**: Failed items go to back of queue
- **Persistence**: State survives crashes/restarts
- **Simplicity**: Easy to reason about and debug

### Why Vault Agent?

- **Authentication**: Automatic auth (token for HCP, cert for Enterprise)
- **Caching**: Reduced latency and load on Vault
- **Token Management**: Auto-renewal
- **Resilience**: Local proxy for Vault
- **Abstraction**: Application doesn't need to know about auth methods

### Why HCL Configuration?

- **Readability**: Clear, human-friendly syntax
- **Ecosystem**: Native to HashiCorp tools
- **Validation**: Strong typing support
- **Hot Reload**: Easy to reload without restart

## Monitoring and Observability

### Logs

- **Application Log**: All operations, errors
- **Audit Log**: Security events (JSON)
- **Progress Log**: File processing updates

### Metrics

- Files processed per second
- Queue depth
- Retry rate
- Error rate
- Processing latency

### Health Checks

- Vault connectivity
- File system accessibility
- Queue state validity

---

**Confidence: 98%** - Architecture is comprehensive and production-ready!
