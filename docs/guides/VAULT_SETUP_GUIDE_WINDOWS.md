# Vault Setup Guide - HCP Vault (Windows)

> **Note**: This guide is for **HCP Vault (cloud-hosted)** on **Windows** platforms. For Unix/Linux/macOS, see [VAULT_SETUP_GUIDE.md](VAULT_SETUP_GUIDE.md). For Vault Enterprise setup, see [VAULT_ENTERPRISE_SETUP_GUIDE_WINDOWS.md](VAULT_ENTERPRISE_SETUP_GUIDE_WINDOWS.md).

This guide walks you through setting up HCP Vault for the file-encryptor application on Windows.

## Overview

The file-encryptor application with HCP Vault uses:
- **HCP Vault** (cloud-hosted) for key management
- **Vault Transit Engine** for envelope encryption
- **Token Authentication** via Vault Agent
- **Vault Agent** as a local proxy with caching

## Prerequisites

Before starting, ensure you have:

1. **HCP Vault Cluster**: Active and accessible
2. **Admin Token**: With permissions to create policies, mount engines, and configure auth
3. **Tools Installed**:
   - [Terraform 1.5+](https://www.terraform.io/downloads)
   - [Vault CLI](https://www.vaultproject.io/downloads)
   - [Git for Windows](https://git-scm.com/download/win) (optional, for Git Bash)

> **Authentication**: HCP Vault uses **token-based authentication** via Vault Agent. Certificate authentication is used for Vault Enterprise (see the Enterprise guide).

## Installation

### Install Terraform

```powershell
# Using Chocolatey (run as Administrator)
choco install terraform

# Or download from: https://www.terraform.io/downloads
# Extract to C:\tools\terraform and add to PATH
```

Verify installation:
```powershell
terraform version
```

### Install Vault CLI

```powershell
# Using Chocolatey (run as Administrator)
choco install vault

# Or download from: https://www.vaultproject.io/downloads
# Extract to C:\tools\vault and add to PATH
```

Verify installation:
```powershell
vault version
```

## Quick Start

### 1. Configure Terraform Variables

```powershell
cd scripts\vault-setup
Copy-Item terraform.tfvars.example terraform.tfvars
notepad terraform.tfvars
```

Edit `terraform.tfvars` with your HCP Vault details:

```hcl
vault_address     = "https://your-vault-cluster.hashicorp.cloud:8200"
vault_namespace   = "admin"
```

### 2. Deploy Vault Configuration

Set your Vault credentials in PowerShell:

```powershell
$env:VAULT_TOKEN = "your-admin-token"
$env:VAULT_ADDR = "https://your-vault-cluster.hashicorp.cloud:8200"
$env:VAULT_NAMESPACE = "admin"
```

Initialize and apply Terraform:

```powershell
cd scripts\vault-setup
terraform init
terraform plan
terraform apply
```

### 3. Verify Configuration

```powershell
# List transit keys
vault list transit/keys

# Read encryption key details
vault read transit/keys/file-encryption-key

# Test data key generation (using your admin token)
vault write -f transit/datakey/plaintext/file-encryption-key
```

### 4. Start Vault Agent

Update the Vault address and namespace in `configs\vault-agent\vault-agent-hcp-token.hcl`, then:

```powershell
# Set your Vault token
$env:VAULT_TOKEN = "your-hcp-admin-token"

# Start Vault Agent (keep this running)
vault agent -config=configs\vault-agent\vault-agent-hcp-token.hcl
```

Leave this running in a PowerShell window. The file-encryptor application will connect to the agent at `http://127.0.0.1:8200`.

> **Note**: For production, use proper token management and consider using HCP Vault's managed authentication methods.

### 5. Test Encryption (New PowerShell Window)

**Encrypt a file:**
```powershell
.\bin\file-encryptor-windows-amd64.exe encrypt `
  -i myfile.txt `
  -o myfile.txt.enc `
  -c configs\examples\example.hcl
```

**Decrypt a file:**
```powershell
.\bin\file-encryptor-windows-amd64.exe decrypt `
  -i myfile.txt.enc `
  -k myfile.txt.key `
  -o myfile-decrypted.txt `
  -c configs\examples\example.hcl
```

## What Gets Created

### Transit Engine
- **Mount Path**: `transit/`
- **Key Name**: `file-encryption-key`
- **Key Type**: AES-256-GCM
- **Configuration**: Non-exportable, deletion protection enabled

### Vault Policy
- **Name**: `file-encryptor-policy`
- **Permissions**:
  - [ALLOW] Generate plaintext data keys
  - [ALLOW] Decrypt data keys
  - [ALLOW] Read key metadata
  - [DENY] Direct encrypt/decrypt (enforces envelope encryption)

## Architecture

```
[File Encryptor]
[ Application  ]
       |
       | http://127.0.0.1:8200
       v
[  Vault Agent   ] <--- Auto-auth with token
[ (Local Proxy)  ]
       |
       | HTTPS + Token
       v
[   HCP Vault    ]
[                ]
[ * Transit Engine]
[ * Policies     ]
[ * Namespace    ]
```

## Security Model

### Envelope Encryption Flow

1. Application requests plaintext data key from Vault
2. Vault generates random DEK and returns both plaintext and encrypted versions
3. Application encrypts file locally with plaintext DEK (AES-256-GCM)
4. Application saves encrypted DEK to `.key` file
5. Application zeros plaintext DEK from memory
6. Only encrypted DEK and ciphertext are stored

### Why Vault Agent?

- **Auto-authentication**: Automatically renews Vault tokens
- **Caching**: Reduces latency and Vault load
- **Simplified config**: Application doesn't need cert management
- **Token management**: Handles token lifecycle

## Windows-Specific Configuration

### Path Separators

Use forward slashes or double backslashes in HCL configuration:

```hcl
# Option 1: Forward slashes (recommended)
encryption {
  source_dir = "C:/data/source"
  dest_dir = "C:/data/encrypted"
}

# Option 2: Double backslashes
encryption {
  source_dir = "C:\\data\\source"
  dest_dir = "C:\\data\\encrypted"
}
```

### File Permissions

Set restrictive permissions using `icacls`:

```powershell
# Queue state file (read/write by current user only)
icacls C:\ProgramData\file-encryptor\queue-state.json /inheritance:r /grant:r "%USERNAME%:(R,W)"

# Log files (read/write by current user only)
icacls C:\ProgramData\file-encryptor\logs\*.log /inheritance:r /grant:r "%USERNAME%:(R,W)"

# Key files (read by current user only)
icacls C:\data\encrypted\*.key /inheritance:r /grant:r "%USERNAME%:R"
```

### Running as a Windows Service

For production, run as a Windows Service using [NSSM](https://nssm.cc/) or [WinSW](https://github.com/winsw/winsw):

**Using NSSM:**
```powershell
# Install NSSM
choco install nssm

# Install service
nssm install FileEncryptor "C:\Program Files\file-encryptor\file-encryptor.exe"
nssm set FileEncryptor AppParameters "watch -c C:\ProgramData\file-encryptor\config.hcl"
nssm set FileEncryptor AppDirectory "C:\Program Files\file-encryptor"
nssm set FileEncryptor DisplayName "File Encryptor Service"
nssm set FileEncryptor Description "Watches directories and encrypts files using Vault Transit Engine"
nssm set FileEncryptor Start SERVICE_AUTO_START

# Start service
Start-Service FileEncryptor

# Check status
Get-Service FileEncryptor
```

## Production Considerations

### 1. Token Management

**Development** (current setup):
- Admin token with broad permissions
- Token stored in environment variable
- Suitable for testing only

**Production**:
- Use HCP Vault's managed authentication
- Implement token rotation
- Follow principle of least privilege
- Use short-lived tokens
- Consider HCP Vault's identity-based authentication

### 2. Key Rotation

Enable automatic key rotation:

```hcl
resource "vault_transit_secret_backend_key" "file_encryption" {
  # ... existing config ...
  auto_rotate_period = 2592000  # 30 days in seconds
}
```

Note: Old versions remain available for decryption.

### 3. High Availability

HCP Vault provides:
- Built-in HA cluster
- Automatic failover
- Regional redundancy

Ensure Vault Agent is configured to handle reconnection.

### 4. Monitoring & Auditing

Monitor Vault operations:

```powershell
# View Vault Agent logs
Get-Content C:\ProgramData\vault-agent\agent.log -Wait

# Monitor application logs
Get-Content C:\ProgramData\file-encryptor\logs\app.log -Wait
```

Enable audit logging in configuration:

```hcl
audit_log {
  enabled = true
  path = "C:/ProgramData/file-encryptor/logs/audit.log"
}
```

### 5. Network Security

Production checklist:
- [ ] Secure token storage and rotation
- [ ] Windows Firewall rules for Vault Agent
- [ ] Use HCP Vault's private endpoints if available
- [ ] Enable TLS for all communications
- [ ] Implement network segmentation

## Troubleshooting

### Token Authentication Fails

**Symptom**: `permission denied` when using token

**Solutions:**
1. Verify token is valid:
   ```powershell
   vault token lookup
   ```
2. Check token has required policies
3. Ensure token is not expired
4. Verify namespace is correct (HCP uses `admin` by default)
5. Check VAULT_TOKEN environment variable is set:
   ```powershell
   $env:VAULT_TOKEN
   ```

### Transit Key Not Found

**Symptom**: `transit key not found`

**Solutions:**
1. Check namespace:
   ```powershell
   vault namespace list
   ```
2. List keys:
   ```powershell
   vault list transit/keys
   ```
3. Verify Terraform applied successfully
4. Check mount path matches config

### Vault Agent Connection Issues

**Symptom**: Agent can't connect to HCP Vault

**Solutions:**
1. Verify `VAULT_ADDR` is correct:
   ```powershell
   $env:VAULT_ADDR
   ```
2. Check network connectivity:
   ```powershell
   Test-NetConnection -ComputerName vault-cluster-xxxxx.hashicorp.cloud -Port 8200
   ```
3. Verify namespace: HCP uses `admin` by default
4. Check Windows Firewall:
   ```powershell
   Get-NetFirewallRule | Where-Object {$_.DisplayName -match "Vault"}
   ```

### Permission Denied on Operations

**Symptom**: Application can authenticate but operations fail

**Solutions:**
1. Check assigned policies:
   ```powershell
   vault token lookup
   ```
2. Verify policy content:
   ```powershell
   vault policy read file-encryptor-policy
   ```
3. Test with Vault CLI using same credentials
4. Check path in policy matches mount path

### Path Issues

**Symptom**: `file not found` or path errors

**Solutions:**
1. Use absolute paths:
   ```powershell
   .\file-encryptor.exe encrypt -i C:\full\path\to\file.txt -o C:\output\file.enc
   ```
2. Escape backslashes in configuration:
   ```hcl
   source_dir = "C:\\data\\source"  # or use forward slashes
   ```
3. Verify directory exists:
   ```powershell
   Test-Path C:\data\source
   ```

## Testing the Setup

### Manual Test: Encrypt/Decrypt Flow

```powershell
# 1. Set environment variables
$env:VAULT_ADDR = "https://your-vault-cluster.hashicorp.cloud:8200"
$env:VAULT_TOKEN = "your-admin-token"
$env:VAULT_NAMESPACE = "admin"

# 2. Generate data key
vault write -f transit/datakey/plaintext/file-encryption-key

# Save the output (you'll get plaintext and ciphertext)

# 3. Decrypt the encrypted DEK
vault write transit/decrypt/file-encryption-key `
  ciphertext="vault:v1:encrypted_dek_from_above"
```

### PowerShell Integration Test

```powershell
# test-vault-setup.ps1
$ErrorActionPreference = "Stop"

Write-Host "Testing Vault setup..."

# Test connectivity
try {
    $health = vault status
    Write-Host "[OK] Vault is accessible" -ForegroundColor Green
} catch {
    Write-Host "[FAILED] Cannot connect to Vault" -ForegroundColor Red
    exit 1
}

# Test data key generation
try {
    $result = vault write -format=json -f transit/datakey/plaintext/file-encryption-key
    Write-Host "[OK] Data key generation successful" -ForegroundColor Green
} catch {
    Write-Host "[FAILED] Data key generation failed" -ForegroundColor Red
    exit 1
}

# Test encryption
"test data" | Out-File -FilePath test.txt -Encoding ASCII -NoNewline
try {
    & .\file-encryptor.exe encrypt -i test.txt -o test.enc
    if ($LASTEXITCODE -eq 0) {
        Write-Host "[OK] File encryption successful" -ForegroundColor Green
    } else {
        throw "Encryption failed"
    }
} catch {
    Write-Host "[FAILED] File encryption failed" -ForegroundColor Red
    exit 1
}

# Cleanup
Remove-Item test.txt, test.enc, test.key -ErrorAction SilentlyContinue

Write-Host "[OK] All tests passed" -ForegroundColor Green
```

## Cleanup

To remove all Vault configuration:

```powershell
cd scripts\vault-setup
terraform destroy
```

## Next Steps

After completing Vault setup:

1. [DONE] Verify Terraform outputs
2. [DONE] Test token authentication
3. [DONE] Confirm Vault Agent is running
4. [DONE] Ready to use the application

Proceed to [CLI Usage (Windows)](CLI_MODE_WINDOWS.md) or [Architecture](../ARCHITECTURE.md) for implementation details.

## Related Guides

- **Vault Enterprise Setup**: See [VAULT_ENTERPRISE_SETUP_GUIDE_WINDOWS.md](VAULT_ENTERPRISE_SETUP_GUIDE_WINDOWS.md)
- **CLI Usage**: See [CLI_MODE_WINDOWS.md](CLI_MODE_WINDOWS.md)
- **Unix/Linux/macOS Guide**: See [VAULT_SETUP_GUIDE.md](VAULT_SETUP_GUIDE.md)

## Additional Resources

- [HCP Vault Documentation](https://developer.hashicorp.com/vault/docs/platform/hcp)
- [Vault Transit Engine](https://developer.hashicorp.com/vault/docs/secrets/transit)
- [Vault Agent](https://developer.hashicorp.com/vault/docs/agent)
- [Envelope Encryption](https://developer.hashicorp.com/vault/tutorials/encryption-as-a-service/eaas-transit)
- [Running Windows Services](https://docs.microsoft.com/en-us/windows/win32/services/services)
