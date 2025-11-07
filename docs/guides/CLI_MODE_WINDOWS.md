# CLI Mode - One-Off Encryption/Decryption (Windows)

This document describes the CLI mode for one-off file encryption and decryption operations on **Windows** platforms.

> For Unix/Linux/macOS instructions, see [CLI_MODE.md](CLI_MODE.md)

## Overview

In addition to the service mode (continuous file watching), the application supports CLI mode for encrypting or decrypting individual files on demand. This is useful for:

- Ad-hoc encryption of sensitive files
- Batch processing via PowerShell scripts
- Testing and validation
- Integration with other tools and workflows

## Commands

### Watch Command (Service Mode)

Runs the continuous file watching service.

```powershell
.\file-encryptor.exe watch -c config.hcl
```

**Options:**
- `-c, --config`: Configuration file path (default: config.hcl)
- `-l, --log-level`: Log level (debug, info, error)
- `-o, --log-output`: Log output (stdout, stderr, or file path)

### Encrypt Command (One-Off)

Encrypts a single file using Vault Transit Engine.

```powershell
.\file-encryptor.exe encrypt -i input.txt -o output.txt.enc
```

**Options:**
- `-i, --input`: Input file to encrypt (required)
- `-o, --output`: Output encrypted file (required)
- `-k, --key`: Output key file (default: `<input>.key`)
- `--checksum`: Calculate and save SHA256 checksum
- `-c, --config`: Configuration file for Vault settings (default: config.hcl)
- `-l, --log-level`: Log level
- `-o, --log-output`: Log output

**Examples:**

```powershell
# Basic encryption
.\file-encryptor.exe encrypt -i sensitive-data.pdf -o sensitive-data.pdf.enc

# Specify custom key file location
.\file-encryptor.exe encrypt -i data.txt -o encrypted\data.txt.enc -k keys\data.txt.key

# Encrypt with checksum
.\file-encryptor.exe encrypt -i document.docx -o document.docx.enc --checksum

# Encrypt with debug logging
.\file-encryptor.exe encrypt -i file.bin -o file.bin.enc -l debug
```

**Output Files:**
- `<output>`: Encrypted file
- `<input>.key`: Encrypted data encryption key (named after input file)
- `<input>.sha256`: Checksum file of original input (if `--checksum` specified)

### Decrypt Command (One-Off)

Decrypts a single file that was encrypted with Vault Transit Engine.

```powershell
.\file-encryptor.exe decrypt -i input.txt.enc -k input.txt.key -o output.txt
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

```powershell
# Basic decryption
.\file-encryptor.exe decrypt -i sensitive-data.pdf.enc -k sensitive-data.pdf.key -o sensitive-data.pdf

# Decrypt with checksum verification
.\file-encryptor.exe decrypt -i document.docx.enc -k document.docx.key -o document.docx --verify-checksum

# Decrypt with debug logging
.\file-encryptor.exe decrypt -i file.bin.enc -k file.bin.key -o file.bin -l debug
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

## Windows-Specific Considerations

### Path Separators

Windows uses backslashes (`\`) for paths, but the application accepts both:

```powershell
# Both work
.\file-encryptor.exe encrypt -i C:\data\file.txt -o C:\encrypted\file.enc
.\file-encryptor.exe encrypt -i C:/data/file.txt -o C:/encrypted/file.enc
```

### File Permissions

On Windows, use `icacls` to set restrictive permissions:

```powershell
# Make key file readable only by current user
icacls data.key /inheritance:r /grant:r "%USERNAME%:R"
```

### PowerShell Execution Policy

If running scripts, you may need to adjust the execution policy:

```powershell
# Check current policy
Get-ExecutionPolicy

# Set to allow local scripts (run as Administrator)
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

### Environment Variables

Set environment variables in PowerShell:

```powershell
$env:VAULT_ADDR = "https://vault.example.com:8200"
$env:VAULT_TOKEN = "your-token-here"
```

Or use System Properties → Environment Variables for persistent settings.

## PowerShell Script Integration

### Batch Encryption Example

```powershell
# encrypt-batch.ps1
# Encrypt all files in a directory

param(
    [string]$SourceDir = "C:\data\source",
    [string]$DestDir = "C:\data\encrypted"
)

$files = Get-ChildItem -Path $SourceDir -File

foreach ($file in $files) {
    $filename = $file.Name
    Write-Host "Encrypting $filename..."
    
    & .\file-encryptor.exe encrypt `
        -i $file.FullName `
        -o "$DestDir\$filename.enc" `
        -k "$DestDir\$filename.key" `
        --checksum
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "[OK] $filename encrypted successfully" -ForegroundColor Green
    } else {
        Write-Host "[FAILED] Failed to encrypt $filename" -ForegroundColor Red
    }
}
```

**Usage:**
```powershell
.\encrypt-batch.ps1 -SourceDir "C:\data\source" -DestDir "C:\data\encrypted"
```

### Batch Decryption Example

```powershell
# decrypt-batch.ps1
# Decrypt all .enc files in a directory

param(
    [string]$SourceDir = "C:\data\encrypted",
    [string]$DestDir = "C:\data\decrypted"
)

$encFiles = Get-ChildItem -Path $SourceDir -Filter "*.enc"

foreach ($encFile in $encFiles) {
    $filename = $encFile.BaseName  # Remove .enc extension
    $keyFile = "$($encFile.FullName.Replace('.enc', '.key'))"
    
    if (-not (Test-Path $keyFile)) {
        Write-Host "[ERROR] Key file not found for $filename" -ForegroundColor Red
        continue
    }
    
    Write-Host "Decrypting $filename..."
    
    & .\file-encryptor.exe decrypt `
        -i $encFile.FullName `
        -k $keyFile `
        -o "$DestDir\$filename" `
        --verify-checksum
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "[OK] $filename decrypted successfully" -ForegroundColor Green
    } else {
        Write-Host "[FAILED] Failed to decrypt $filename" -ForegroundColor Red
    }
}
```

**Usage:**
```powershell
.\decrypt-batch.ps1 -SourceDir "C:\data\encrypted" -DestDir "C:\data\decrypted"
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
Solution: Verify the input file path is correct. Use absolute paths or ensure current directory is correct.
Example: .\file-encryptor.exe encrypt -i C:\full\path\to\file.txt -o C:\output\file.enc
```

**Error**: `failed to load configuration`
```
Solution: Ensure config file exists and contains valid Vault settings
Example: .\file-encryptor.exe encrypt -i file.txt -o file.enc -c C:\path\to\config.hcl
```

**Error**: `vault health check failed`
```
Solution: Ensure Vault Agent is running and accessible at the configured address
Check: netstat -an | findstr "8200" to verify port is listening
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

### PowerShell Integration Test

```powershell
# test-cli.ps1
# Integration test for CLI mode

$ErrorActionPreference = "Stop"

Write-Host "Testing CLI mode..."

# Test encryption
"secret data" | Out-File -FilePath test.txt -Encoding ASCII -NoNewline
.\file-encryptor.exe encrypt -i test.txt -o test.txt.enc --checksum

if ($LASTEXITCODE -ne 0) {
    Write-Host "[ERROR] Encryption failed" -ForegroundColor Red
    exit 1
}
Write-Host "[OK] Encryption successful" -ForegroundColor Green

# Verify files exist
if (-not (Test-Path test.txt.enc)) {
    Write-Host "[ERROR] Encrypted file not created" -ForegroundColor Red
    exit 1
}
Write-Host "[OK] Encrypted file created" -ForegroundColor Green

if (-not (Test-Path test.txt.key)) {
    Write-Host "[ERROR] Key file not created" -ForegroundColor Red
    exit 1
}
Write-Host "[OK] Key file created" -ForegroundColor Green

if (-not (Test-Path test.txt.sha256)) {
    Write-Host "[ERROR] Checksum file not created" -ForegroundColor Red
    exit 1
}
Write-Host "[OK] Checksum file created" -ForegroundColor Green

# Test decryption
.\file-encryptor.exe decrypt -i test.txt.enc -k test.txt.key -o test-decrypted.txt --verify-checksum

if ($LASTEXITCODE -ne 0) {
    Write-Host "[ERROR] Decryption failed" -ForegroundColor Red
    exit 1
}
Write-Host "[OK] Decryption successful" -ForegroundColor Green

# Verify content
$original = Get-Content test.txt -Raw
$decrypted = Get-Content test-decrypted.txt -Raw

if ($original -eq $decrypted) {
    Write-Host "[OK] Decrypted content matches original" -ForegroundColor Green
} else {
    Write-Host "[ERROR] Decrypted content does not match" -ForegroundColor Red
    exit 1
}

# Cleanup
Remove-Item test.txt, test.txt.enc, test.txt.key, test.txt.sha256, test-decrypted.txt

Write-Host "[OK] All CLI tests passed" -ForegroundColor Green
```

**Usage:**
```powershell
.\test-cli.ps1
```

## Performance Considerations

- **Streaming**: Files are processed in 1MB chunks, memory usage is constant regardless of file size
- **Progress**: Updates logged every 20% to track long-running operations
- **No Queue**: CLI mode bypasses the queue system for immediate processing
- **Single Operation**: Each command processes exactly one file then exits

## Security Considerations

1. **Key File Security**: The `.key` files contain encrypted DEKs and should be stored securely
2. **Permissions**: Use `icacls` to set restrictive permissions on encrypted files and keys
3. **Cleanup**: Ensure the original file is handled securely after encryption (delete or move to secure location)
4. **Logging**: Be careful not to log sensitive file paths or content
5. **Memory**: Plaintext DEKs are zeroed from memory after use

## Windows Service Mode

For running as a Windows Service, hot-reload is not supported. To change configuration:

1. Stop the service
2. Update configuration file
3. Restart the service

**Stopping service (PowerShell as Administrator):**
```powershell
Stop-Process -Name "file-encryptor" -Force
```

Or press `Ctrl+C` if running in foreground.

## Comparison: CLI Mode vs Service Mode

| Feature | CLI Mode | Service Mode |
|---------|----------|--------------|
| Use Case | One-off operations | Continuous monitoring |
| Queue | No | Yes (FIFO with persistence) |
| Retry Logic | No | Yes (exponential backoff) |
| File Watching | No | Yes (fsnotify) |
| Configuration | Minimal (Vault only) | Full (directories, queue, etc.) |
| Graceful Shutdown | N/A (exits immediately) | Yes (saves queue state) |
| Hot Reload | No | No (Windows limitation) |
| Batch Processing | Via PowerShell scripts | Automatic |
| Progress Reporting | Logged to console | Logged to file/stdout |

## Exit Codes

- `0`: Success
- `1`: General error (file not found, encryption/decryption failed, etc.)
- `2`: Configuration error
- `3`: Vault connection error

Use in PowerShell:
```powershell
.\file-encryptor.exe encrypt -i file.txt -o file.enc
if ($LASTEXITCODE -eq 0) {
    Write-Host "Success"
} else {
    Write-Host "Failed with exit code $LASTEXITCODE"
}
```

## Additional Resources

- [Windows Installation Guide](../../README.md#windows)
- [Configuration Guide](../ARCHITECTURE.md#configuration-management)
- [Vault Setup for Windows](VAULT_SETUP_GUIDE_WINDOWS.md)
- [Main CLI Mode Guide](CLI_MODE.md) (Unix/Linux/macOS)

---

For questions specific to Windows deployment, please open an issue on GitHub with the `windows` label.
