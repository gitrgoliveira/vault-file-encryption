# CLI Mode - One-Off Encryption/Decryption

This document describes the CLI mode for one-off file encryption and decryption operations on **Unix, Linux, and macOS** platforms.

> **For Windows users**, see [CLI_MODE_WINDOWS.md](CLI_MODE_WINDOWS.md) for PowerShell examples and Windows-specific instructions.

## Overview

In addition to the service mode (continuous file watching), the application supports CLI mode for encrypting or decrypting individual files on demand. This is useful for:

- Ad-hoc encryption of sensitive files
- Batch processing via shell scripts
- Testing and validation
- Integration with other tools and workflows

## Commands

### Watch Command (Service Mode)

Runs the continuous file watching service.

```bash
file-encryptor watch -c config.hcl
```

**Options:**
- `-c, --config`: Configuration file path (default: config.hcl)
- `-l, --log-level`: Log level (debug, info, error)
- `-o, --log-output`: Log output (stdout, stderr, or file path)

### Encrypt Command (One-Off)

Encrypts a single file using Vault Transit Engine.

```bash
file-encryptor encrypt -i input.txt -o output.txt.enc
```

**Options:**
- `-i, --input`: Input file to encrypt (required)
- `-o, --output`: Output encrypted file (required)
- `-k, --key`: Output key file (default: `<output>.key`)
- `--checksum`: Calculate and save SHA256 checksum
- `-c, --config`: Configuration file for Vault settings (default: config.hcl)
- `-l, --log-level`: Log level
- `-o, --log-output`: Log output

**Examples:**

```bash
# Basic encryption
file-encryptor encrypt -i sensitive-data.pdf -o sensitive-data.pdf.enc

# Specify custom key file location
file-encryptor encrypt -i data.txt -o encrypted/data.txt.enc -k keys/data.txt.key

# Encrypt with checksum
file-encryptor encrypt -i document.docx -o document.docx.enc --checksum

# Encrypt with debug logging
file-encryptor encrypt -i file.bin -o file.bin.enc -l debug
```

**Output Files:**
- `<output>`: Encrypted file
- `<output>.key` or `<key>`: Encrypted data encryption key
- `<output>.sha256`: Checksum file (if `--checksum` specified)

### Decrypt Command (One-Off)

Decrypts a single file that was encrypted with Vault Transit Engine.

```bash
file-encryptor decrypt -i input.txt.enc -k input.txt.key -o output.txt
```

**Options:**
- `-i, --input`: Encrypted file to decrypt (required)
- `-k, --key`: Key file (required)
- `-o, --output`: Output decrypted file (required)
- `--verify-checksum`: Verify SHA256 checksum if available
- `-c, --config`: Configuration file for Vault settings (default: config.hcl)
- `-l, --log-level`: Log level
- `-o, --log-output`: Log output

**Examples:**

```bash
# Basic decryption
file-encryptor decrypt -i sensitive-data.pdf.enc -k sensitive-data.pdf.key -o sensitive-data.pdf

# Decrypt with checksum verification
file-encryptor decrypt -i document.docx.enc -k document.docx.key -o document.docx --verify-checksum

# Decrypt with debug logging
file-encryptor decrypt -i file.bin.enc -k file.bin.key -o file.bin -l debug
```

## Configuration Requirements

CLI mode requires minimal configuration - only Vault connection settings are needed.

### Minimal Configuration for CLI Mode

```hcl
# config.hcl - Minimal configuration for CLI mode

vault {
  agent_address = "http://127.0.0.1:8200"
  transit_mount = "transit"
  key_name = "file-encryption-key"
}

# Logging configuration (optional)
logging {
  level = "info"
  format = "text"
}
```

The following configuration sections are **NOT required** for CLI mode:
- `encryption` (source/dest directories)
- `decryption` (source/dest directories)
- `queue` (state path, retries)

## Implementation Details

### CLI Flow for Encryption

```go
func runEncrypt(inputFile, outputFile, keyFile string, calculateChecksum bool) error {
    // 1. Initialize logger
    log := logger.New(logLevel, logOutput)
    
    // 2. Load configuration (Vault settings only)
    cfg := config.Load(configFile)
    
    // 3. Verify input file exists
    if _, err := os.Stat(inputFile); os.IsNotExist(err) {
        return fmt.Errorf("input file not found: %s", inputFile)
    }
    
    // 4. Set default key file if not specified
    if keyFile == "" {
        keyFile = outputFile + ".key"
    }
    
    // 5. Initialize Vault client
    vaultClient := vault.NewClient(cfg.Vault)
    
    // 6. Calculate checksum (if requested)
    var checksum string
    if calculateChecksum {
        checksum = crypto.CalculateChecksum(inputFile)
        crypto.SaveChecksum(checksum, outputFile + ".sha256")
    }
    
    // 7. Encrypt file
    encryptor := crypto.NewEncryptor(vaultClient)
    progressCallback := func(progress float64) {
        log.Info("Encryption progress", "percent", progress)
    }
    encryptedKey, err := encryptor.EncryptFile(inputFile, outputFile, progressCallback)
    if err != nil {
        return fmt.Errorf("encryption failed: %w", err)
    }
    
    // 8. Save encrypted key
    os.WriteFile(keyFile, []byte(encryptedKey), 0600)
    
    // 9. Log success
    log.Info("File encrypted successfully",
        "input", inputFile,
        "output", outputFile,
        "key", keyFile,
    )
    
    return nil
}
```

### CLI Flow for Decryption

```go
func runDecrypt(inputFile, keyFile, outputFile string, verifyChecksum bool) error {
    // 1. Initialize logger
    log := logger.New(logLevel, logOutput)
    
    // 2. Load configuration (Vault settings only)
    cfg := config.Load(configFile)
    
    // 3. Verify input files exist
    if _, err := os.Stat(inputFile); os.IsNotExist(err) {
        return fmt.Errorf("encrypted file not found: %s", inputFile)
    }
    if _, err := os.Stat(keyFile); os.IsNotExist(err) {
        return fmt.Errorf("key file not found: %s", keyFile)
    }
    
    // 4. Initialize Vault client
    vaultClient := vault.NewClient(cfg.Vault)
    
    // 5. Decrypt file
    decryptor := crypto.NewDecryptor(vaultClient)
    progressCallback := func(progress float64) {
        log.Info("Decryption progress", "percent", progress)
    }
    err := decryptor.DecryptFile(inputFile, keyFile, outputFile, progressCallback)
    if err != nil {
        return fmt.Errorf("decryption failed: %w", err)
    }
    
    // 6. Verify checksum (if requested and available)
    if verifyChecksum {
        checksumFile := strings.TrimSuffix(inputFile, ".enc") + ".sha256"
        if _, err := os.Stat(checksumFile); err == nil {
            expectedChecksum := crypto.LoadChecksum(checksumFile)
            valid, err := crypto.VerifyChecksum(outputFile, expectedChecksum)
            if err != nil {
                log.Warn("Checksum verification failed", "error", err)
            } else if !valid {
                return fmt.Errorf("checksum verification failed")
            }
            log.Info("Checksum verified successfully")
        } else {
            log.Warn("No checksum file found, skipping verification")
        }
    }
    
    // 7. Log success
    log.Info("File decrypted successfully",
        "input", inputFile,
        "output", outputFile,
    )
    
    return nil
}
```

## Exit Codes

- `0`: Success
- `1`: General error (file not found, encryption/decryption failed, etc.)
- `2`: Configuration error
- `3`: Vault connection error

## Shell Script Integration

### Batch Encryption Example

```bash
#!/bin/bash
# Encrypt all files in a directory

SOURCE_DIR="/path/to/source"
DEST_DIR="/path/to/encrypted"

for file in "$SOURCE_DIR"/*; do
    filename=$(basename "$file")
    echo "Encrypting $filename..."
    file-encryptor encrypt \
        -i "$file" \
        -o "$DEST_DIR/$filename.enc" \
        -k "$DEST_DIR/$filename.key" \
        --checksum
    
    if [ $? -eq 0 ]; then
        echo "[OK] $filename encrypted successfully"
    else
        echo "[FAILED] Failed to encrypt $filename" >&2
    fi
done
```

### Batch Decryption Example

```bash
#!/bin/bash
# Decrypt all .enc files in a directory

SOURCE_DIR="/path/to/encrypted"
DEST_DIR="/path/to/decrypted"

for enc_file in "$SOURCE_DIR"/*.enc; do
    filename=$(basename "$enc_file" .enc)
    key_file="${enc_file%.enc}.key"
    
    if [ ! -f "$key_file" ]; then
        echo "[ERROR] Key file not found for $filename" >&2
        continue
    fi
    
    echo "Decrypting $filename..."
    file-encryptor decrypt \
        -i "$enc_file" \
        -k "$key_file" \
        -o "$DEST_DIR/$filename" \
        --verify-checksum
    
    if [ $? -eq 0 ]; then
        echo "[OK] $filename decrypted successfully"
    else
        echo "[FAILED] Failed to decrypt $filename" >&2
    fi
done
```

## Progress Reporting

For large files, progress is logged every 20%:

```
2025-11-05T10:30:45Z [INFO] Encrypting file input=largefile.bin output=largefile.bin.enc
2025-11-05T10:30:46Z [INFO] Encryption progress percent=20
2025-11-05T10:30:47Z [INFO] Encryption progress percent=40
2025-11-05T10:30:48Z [INFO] Encryption progress percent=60
2025-11-05T10:30:49Z [INFO] Encryption progress percent=80
2025-11-05T10:30:50Z [INFO] Encryption progress percent=100
2025-11-05T10:30:50Z [INFO] File encrypted successfully input=largefile.bin output=largefile.bin.enc
```

## Error Handling

### Common Errors and Solutions

**Error**: `input file not found`
```
Solution: Verify the input file path is correct and the file exists
```

**Error**: `failed to load configuration`
```
Solution: Ensure config file exists and contains valid Vault settings
```

**Error**: `vault health check failed`
```
Solution: Ensure Vault Agent is running and accessible at the configured address
```

**Error**: `failed to generate data key`
```
Solution: Check Vault connectivity and ensure the transit key exists and you have permission
```

**Error**: `checksum verification failed`
```
Solution: The decrypted file may be corrupted or tampered with
```

## Testing CLI Mode

### Unit Tests

```go
func TestEncryptCommand(t *testing.T) {
    // Create test file
    tmpFile := createTestFile(t, "test content")
    defer os.Remove(tmpFile)
    
    // Run encrypt command
    err := runEncrypt(tmpFile, tmpFile+".enc", "", false)
    require.NoError(t, err)
    
    // Verify outputs exist
    assert.FileExists(t, tmpFile+".enc")
    assert.FileExists(t, tmpFile+".enc.key")
}

func TestDecryptCommand(t *testing.T) {
    // Setup encrypted file
    encFile, keyFile := setupEncryptedFile(t)
    defer cleanup(encFile, keyFile)
    
    // Run decrypt command
    outFile := t.TempDir() + "/decrypted.txt"
    err := runDecrypt(encFile, keyFile, outFile, false)
    require.NoError(t, err)
    
    // Verify decrypted content
    content, _ := os.ReadFile(outFile)
    assert.Equal(t, "test content", string(content))
}
```

### Integration Test

```bash
#!/bin/bash
# Integration test for CLI mode

set -e

echo "Testing CLI mode..."

# Test encryption
echo "secret data" > test.txt
file-encryptor encrypt -i test.txt -o test.txt.enc --checksum
echo "[OK] Encryption successful"

# Verify files exist
[ -f test.txt.enc ] && echo "[OK] Encrypted file created"
[ -f test.txt.enc.key ] && echo "[OK] Key file created"
[ -f test.txt.enc.sha256 ] && echo "[OK] Checksum file created"

# Test decryption
file-encryptor decrypt -i test.txt.enc -k test.txt.enc.key -o test-decrypted.txt --verify-checksum
echo "[OK] Decryption successful"

# Verify content
if diff test.txt test-decrypted.txt > /dev/null; then
    echo "[OK] Decrypted content matches original"
else
    echo "[ERROR] Decrypted content does not match" >&2
    exit 1
fi

# Cleanup
rm test.txt test.txt.enc test.txt.enc.key test.txt.enc.sha256 test-decrypted.txt
echo "[OK] All CLI tests passed"
```

## Performance Considerations

- **Streaming**: Files are processed in 1MB chunks, memory usage is constant regardless of file size
- **Progress**: Updates logged every 20% to track long-running operations
- **No Queue**: CLI mode bypasses the queue system for immediate processing
- **Single Operation**: Each command processes exactly one file then exits

## Security Considerations

1. **Key File Security**: The `.key` files contain encrypted DEKs and should be stored securely
2. **Permissions**: Consider setting restrictive permissions on encrypted files and keys
3. **Cleanup**: Ensure the original file is handled securely after encryption (delete or move to secure location)
4. **Logging**: Be careful not to log sensitive file paths or content
5. **Memory**: Plaintext DEKs are zeroed from memory after use

## Comparison: CLI Mode vs Service Mode

| Feature | CLI Mode | Service Mode |
|---------|----------|--------------|
| Use Case | One-off operations | Continuous monitoring |
| Queue | No | Yes (FIFO with persistence) |
| Retry Logic | No | Yes (exponential backoff) |
| File Watching | No | Yes (fsnotify) |
| Configuration | Minimal (Vault only) | Full (directories, queue, etc.) |
| Graceful Shutdown | N/A (exits immediately) | Yes (saves queue state) |
| Hot Reload | No | Yes (Unix/Linux/macOS only) |
| Batch Processing | Via shell scripts | Automatic |
| Progress Reporting | Logged to console | Logged to file/stdout |

## Platform-Specific Documentation

- **Unix/Linux/macOS**: This guide
- **Windows**: See [CLI_MODE_WINDOWS.md](CLI_MODE_WINDOWS.md)

## Adding to Documentation

Update the main README.md with CLI usage:

```markdown
## Usage

### Service Mode (Continuous Watching)

```bash
# Start the file watcher service
file-encryptor watch -c config.hcl
```

### CLI Mode (One-Off Operations)

```bash
# Encrypt a single file
file-encryptor encrypt -i input.txt -o output.txt.enc --checksum

# Decrypt a single file
file-encryptor decrypt -i output.txt.enc -k output.txt.key -o decrypted.txt --verify-checksum
```

For detailed CLI usage, see [CLI Mode Documentation](CLI_MODE.md).
```

---

This CLI mode provides flexibility for both automated service-based encryption and manual/scripted operations, making the tool suitable for a wider range of use cases.
