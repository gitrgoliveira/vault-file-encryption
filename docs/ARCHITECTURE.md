# Architecture

This document describes the architecture of the Vault File Encryption tool, including its core components, data flows, and security model.

## Overview

The application provides secure file encryption using HashiCorp Vault's Transit secrets engine. It implements **envelope encryption**, where a unique Data Encryption Key (DEK) is generated for each file and encrypted by Vault's primary key.

### Operation Modes

| Mode | Command | Description |
|------|---------|-------------|
| **Service Mode** | `watch` | Continuously monitors directories for new files |
| **CLI Mode** | `encrypt` / `decrypt` | One-off encryption or decryption of individual files |

### High-Level Architecture

```mermaid
graph TD
    subgraph App["File Encryptor Application"]
        subgraph CLI_Interface["CLI Interface"]
            direction TB
            watch["watch (service)"]
            encrypt["encrypt (one-off)"]
            decrypt["decrypt (one-off)"]
        end

        file_watcher["File Watcher (fsnotify)"]
        queue["FIFO Queue (Persistent)"]
        processor["Direct Processor"]
        retry["Retry Logic (Exp Backoff)"]

        subgraph Crypto["Crypto Package"]
            Encryptor
            Decryptor
            Checksum
        end

        subgraph VaultClient["Vault Client"]
            DK_Ops["Data Key Operations"]
        end
    end

    watch --> file_watcher
    encrypt --> processor
    decrypt --> processor

    file_watcher --> retry
    retry --> queue
    queue --> processor
    
    processor --> Encryptor
    processor --> Decryptor
    processor --> Checksum

    Encryptor --> DK_Ops
    Decryptor --> DK_Ops

    DK_Ops --> Agent["Vault Agent"]
    Agent --> Vault["HCP Vault / Vault Enterprise"]
```

---

## Data Flow

### Encryption Flow

```mermaid
graph TD
    NewFile(["New File"]) --> Watcher["File Watcher"]
    Watcher -->|"Stability Check"| Queue["Enqueue Item"]
    Queue --> FIFO[("FIFO Queue")]
    FIFO --> Proc["Processor"]
    
    Proc --> Checksum{"Calc Checksum?"}
    Checksum -- Yes --> SaveSum["Save .sha256"]
    Checksum -- No --> ReqKey
    SaveSum --> ReqKey["Request Data Key"]
    
    ReqKey -->|"POST /transit/datakey"| Vault[("Vault")]
    Vault -->|"plaintext DEK"| Enc["Encrypt File"]
    
    Enc -->|"Write .enc"| Dest[("Destination")]
    Enc -->|"Save .key"| SaveKey["Save Ciphertext DEK"]
    
    SaveKey --> Zero["Zero Plaintext DEK"]
    Zero --> HandleSource{"Handle Source"}
    
    HandleSource -->|"Archive"| Archive["Move to archive/"]
    HandleSource -->|"Delete"| Delete["Remove file"]
```

### Decryption Flow

```mermaid
graph TD
    NewPair(["Encrypted Pair Detected"]) --> Watcher["File Watcher"]
    Watcher --> Queue["Enqueue Item"]
    Queue --> FIFO[("FIFO Queue")]
    FIFO --> Proc["Processor"]
    
    Proc --> ReadKey["Read .key file"]
    ReadKey --> DecKey["Decrypt DEK"]
    
    DecKey -->|"POST /transit/decrypt"| Vault[("Vault")]
    Vault -->|"plaintext DEK"| DecFile["Decrypt File"]
    
    DecFile --> Verify{"Verify Checksum?"}
    Verify -- Yes --> CompSum["Compare .sha256"]
    Verify -- No --> Zero
    CompSum --> Zero["Zero Plaintext DEK"]
    
    Zero --> HandleSource{"Handle Source"}
    HandleSource -->|"Archive"| Archive["Move to archive/"]
    HandleSource -->|"Delete"| Delete["Remove files"]
```

---

## Security Architecture

### Envelope Encryption

The application uses envelope encryption, a best practice for securing data at scale:

1. **Primary Key** — Stored securely in Vault, never leaves the HSM/server
2. **Data Encryption Key (DEK)** — Generated per-file, used locally for encryption
3. **Ciphertext DEK** — The DEK encrypted by the primary key, safe to store

```mermaid
graph TD
    Primary["Primary Key (Vault Transit)"]
    
    Primary -->|"Encrypts/Decrypts"| DEK1["Data Encryption Key 1"]
    Primary --> DEK2["Data Encryption Key 2"]
    Primary --> DEK_More["Data Encryption Key N..."]
    
    DEK1 --> Plaintext["Plaintext DEK (in memory only)"]
    DEK1 --> Ciphertext["Ciphertext DEK (stored in .key file)"]
```

### Encrypted File Format

```
.enc File:
┌──────────────┬─────────┬──────────────┬──────────────┬─────┐
│ Magic Header │ Version │ Salt         │ Chunk1       │ ... │
│ (4 bytes)    │ (1 byte)│ (32 bytes)   │ (ciphertext) │     │
└──────────────┴─────────┴──────────────┴──────────────┴─────┘
     │              │           │              │
     │              │           │              └─ Encrypted with AES-256-GCM
     │              │           └──────────────── Argon2id/PBKDF2 salt
     │              └──────────────────────────── Format version (v1)
     └─────────────────────────────────────────── File signature

.key File:
vault:v{version}:{base64-encrypted-DEK}
```

### Security Features

| Feature | Description |
|---------|-------------|
| **AES-256-GCM** | Authenticated encryption with 256-bit keys |
| **Unique DEK per file** | Limits blast radius if a key is compromised |
| **Memory zeroing** | Plaintext keys are securely zeroed after use |
| **Integrity verification** | GCM authentication tags and optional SHA-256 checksums |
| **Vault Agent** | Local proxy for caching, authentication, and token renewal |

---

## Error Handling

### Retry Strategy

Failed operations are automatically retried with exponential backoff:

```mermaid
graph TD
    Attempt["Processing Attempt"] --> Check{"Success?"}
    Check -- Yes --> Complete["Mark Complete"] --> Handle["Handle Source File"]
    Check -- No --> Count{"Check Retry Count"}
    
    Count -->|"< Max Retries"| Calc["Calculate Backoff"]
    Calc --> Delay["delay = base * 2^attempts"]
    Delay --> Fail["Mark Failed"]
    Fail --> Requeue["Requeue to end of FIFO"]
    
    Count -->|">= Max Retries"| DLQ["Mark as DLQ"]
    DLQ --> MoveDLQ["Move to .dlq/ folder"]
```

### Dead Letter Queue

Files that fail after all retries are moved to the `.dlq/` directory for manual investigation.

---

## Deployment

### HCP Vault

```mermaid
graph TD
    subgraph Host["Host/VM"]
        FE["file-encryptor (systemd)"]
        VA["vault-agent (systemd)"]
        
        FE -- "HTTP localhost" --> VA
    end
    
    VA -- "HTTPS + Token" --> HCP["HCP Vault"]
```

### Vault Enterprise

```mermaid
graph TD
    subgraph Host["Host/VM"]
        FE["file-encryptor (systemd)"]
        VA["vault-agent (systemd)"]
        
        FE -- "HTTP localhost" --> VA
    end
    
    VA -- "HTTPS + Cert" --> Ent["Vault Enterprise"]
```

---

## Configuration

### Hot Reload

Configuration changes can be applied without restarting the service by sending a `SIGHUP` signal:

```bash
kill -HUP $(pidof file-encryptor)
```

The application will:
1. Load and validate the new configuration
2. Apply changes if valid (or keep the current config if invalid)
3. Update all affected components (Vault client, watcher paths, queue settings)

### Graceful Shutdown

On receiving `SIGTERM` or `SIGINT`, the application:

1. Stops accepting new files
2. Saves the current queue state to disk
3. Flushes logs
4. Exits cleanly

On restart, the queue state is restored and processing resumes.

---

## Performance

| Aspect | Implementation |
|--------|----------------|
| **Streaming** | Files processed in 1MB chunks to limit memory usage |
| **Caching** | Vault Agent caches responses to reduce latency |
| **Sequential Processing** | Single processor ensures predictable ordering |
| **Progress Reporting** | Updates logged every 20% for large files |
