# Vault Enterprise Setup Guide - Local Dev Mode

> **Note**: This guide is for **Vault Enterprise (self-hosted)** on **Unix, Linux, and macOS** platforms. For Windows, see [VAULT_ENTERPRISE_SETUP_GUIDE_WINDOWS.md](VAULT_ENTERPRISE_SETUP_GUIDE_WINDOWS.md). For HCP Vault setup, see [VAULT_SETUP_GUIDE.md](VAULT_SETUP_GUIDE.md).

This guide walks you through setting up a local Vault Enterprise instance in dev mode for the file-encryptor application.

## Overview

The file-encryptor application with Vault Enterprise uses:
- **Vault Enterprise** (self-hosted, dev mode) for key management
- **Vault Transit Engine** for envelope encryption
- **Certificate Authentication** via Vault Agent
- **Vault Agent** as a local proxy with caching

## Important: Dev Mode vs Production

**This guide uses Vault dev mode**, which is designed for development and testing ONLY:

- ✓ Quick to start (no configuration needed)
- ✓ Auto-unsealed
- ✓ In-memory storage
- ✗ **Data lost on restart**
- ✗ **NOT secure for production**
- ✗ **No persistence**

For production Vault Enterprise deployments, consult the [Vault Enterprise Production Deployment Guide](https://developer.hashicorp.com/vault/tutorials/operations/production-hardening).

## Prerequisites

Before starting, ensure you have:

1. **Vault Binary**: Vault Community Edition or Enterprise Edition installed
   - Download from: https://developer.hashicorp.com/vault/downloads
   - Verify: `vault version`
2. **OpenSSL**: For certificate generation
   - Verify: `openssl version`

> **Authentication**: Vault Enterprise uses **certificate-based authentication** via Vault Agent. Token authentication is used for HCP Vault (see the HCP guide).

## Quick Start

### 1. Start Vault in Dev Mode

```bash
cd scripts/vault-setup-enterprise
./01-start-vault-dev.sh
```

This starts Vault on `http://127.0.0.1:8200` with a root token.

### 2. Configure Vault

In a new terminal:

```bash
cd scripts/vault-setup-enterprise
./02-configure-vault.sh
```

This script:
- Enables Transit engine
- Creates encryption key
- Generates test certificates
- Configures certificate authentication
- Creates application policy

### 3. Test the Setup

```bash
cd scripts/vault-setup-enterprise
./03-test-setup.sh
```

This verifies:
- Certificate authentication works
- Data key generation works
- Policy permissions are correct

### 4. Start Vault Agent

In a new terminal:

```bash
vault agent -config=configs/vault-agent/vault-agent-enterprise-dev.hcl
```

Leave this running. The file-encryptor application will connect to the agent at `http://127.0.0.1:8210`.

> **Note**: Enterprise Vault Agent uses port **8210** (not 8200) to avoid conflicts with HCP Vault Agent, if it's running.

### 5. Run File Encryptor

```bash
# Encrypt a file
./bin/file-encryptor encrypt \
  -i test.txt \
  -o test.txt.enc \
  -c configs/examples/example-enterprise.hcl

# Decrypt a file
./bin/file-encryptor decrypt \
  -i test.txt.enc \
  -k test.txt.key \
  -o test-decrypted.txt \
  -c configs/examples/example-enterprise.hcl
```

## What Gets Created

### Transit Engine
- **Mount Path**: `transit/`
- **Key Name**: `file-encryption-key`
- **Key Type**: AES-256-GCM
- **Configuration**: Non-exportable, deletion protection enabled

### Certificate Authentication
- **Mount Path**: `auth/cert/`
- **Role Name**: `file-encryptor`
- **Allowed CN**: `file-encryptor.local`
- **CA Certificate**: Self-signed (test only)
- **Client Certificate**: Generated locally

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
       | http://127.0.0.1:8210
       v
[  Vault Agent   ] <--- Auto-auth with client cert
[ (Local Proxy)  ]
       |
       | HTTP (dev mode, no TLS)
       v
[Vault Dev Server]
[ localhost:8200 ]
[                ]
[ * Transit Engine]
[ * Cert Auth    ]
[ * Policies     ]
```

## Security Model

### Envelope Encryption Flow

1. Application requests plaintext data key from Vault
2. Vault generates random DEK and returns both plaintext and encrypted versions
3. Application encrypts file locally with plaintext DEK (AES-256-GCM)
4. Application saves encrypted DEK to `.key` file
5. Application zeros plaintext DEK from memory
6. Only encrypted DEK and ciphertext are stored

### Certificate Authentication

- Vault Agent authenticates using client certificate
- Client certificate signed by CA uploaded to Vault
- Certificate CN must match allowed list in Vault role
- Vault Agent manages token lifecycle
- Application never handles certificates directly

## Detailed Setup Steps

### Step 1: Start Vault Dev Server

The `01-start-vault-dev.sh` script does:

```bash
# Start Vault in dev mode
vault server -dev \
  -dev-root-token-id="dev-root-token" \
  -dev-listen-address="127.0.0.1:8200"
```

Key features:
- Root token: `dev-root-token` (configurable)
- Address: `http://127.0.0.1:8200`
- Auto-unsealed
- In-memory storage
- HTTP only (no TLS in dev mode)

### Step 2: Configure Transit and Auth

The `02-configure-vault.sh` script does:

1. **Enable Transit Engine**:
   ```bash
   vault secrets enable -path=transit transit
   ```

2. **Create Encryption Key**:
   ```bash
   vault write -f transit/keys/file-encryption-key \
     type=aes256-gcm96 \
     deletion_allowed=false \
     exportable=false
   ```

3. **Generate Certificates** (reuses existing script):
   ```bash
   cd ../../test-certs
   ./generate-certs.sh
   cd ../vault-setup-enterprise
   ```

4. **Configure Certificate Auth**:
   ```bash
   # Enable cert auth
   vault auth enable cert
   
   # Upload CA certificate
   vault write auth/cert/certs/file-encryptor \
     certificate=@../../test-certs/ca.crt \
     allowed_common_names="file-encryptor.local" \
     token_policies="file-encryptor-policy"
   ```

5. **Create Policy**:
   ```bash
   vault policy write file-encryptor-policy - <<EOF
   # Allow generating data keys
   path "transit/datakey/plaintext/file-encryption-key" {
     capabilities = ["create", "update"]
   }
   
   # Allow decrypting data keys
   path "transit/decrypt/file-encryption-key" {
     capabilities = ["create", "update"]
   }
   
   # Deny direct encrypt/decrypt
   path "transit/encrypt/file-encryption-key" {
     capabilities = ["deny"]
   }
   EOF
   ```

### Step 3: Test Setup

The `03-test-setup.sh` script verifies:

1. **Certificate Authentication**:
   ```bash
   vault login -method=cert \
     -client-cert=../../test-certs/client.crt \
     -client-key=../../test-certs/client-key.pem
   ```

2. **Data Key Generation**:
   ```bash
   vault write -f transit/datakey/plaintext/file-encryption-key
   ```

3. **Data Key Decryption**:
   ```bash
   vault write transit/decrypt/file-encryption-key \
     ciphertext="vault:v1:..."
   ```

## Troubleshooting

### Vault Dev Server Won't Start

**Symptom**: `address already in use`

**Solutions**:
1. Check if Vault is already running: `ps aux | grep vault`
2. Kill existing process: `pkill vault`
3. Check port 8200: `lsof -i :8200`
4. Start on different port: `vault server -dev -dev-listen-address="127.0.0.1:8201"`

### Certificate Authentication Fails

**Symptom**: `permission denied` when using cert auth

**Solutions**:
1. Verify CA cert uploaded: `vault read auth/cert/certs/file-encryptor`
2. Check certificate CN: `openssl x509 -in scripts/test-certs/client.crt -noout -subject`
3. Should show: `CN = file-encryptor.local`
4. Ensure certificate not expired: `openssl x509 -in scripts/test-certs/client.crt -noout -dates`
5. Regenerate certificates: `cd scripts/test-certs && ./generate-certs.sh`

### Vault Agent Connection Issues

**Symptom**: Agent can't connect to Vault

**Solutions**:
1. Verify Vault is running: `vault status`
2. Check address: `http://127.0.0.1:8200`
3. Verify in vault-agent config: `vault.address = "http://127.0.0.1:8200"`
4. Check Vault Agent logs for errors

### Permission Denied on Operations

**Symptom**: Application can authenticate but operations fail

**Solutions**:
1. Check assigned policies: `vault token lookup`
2. Verify policy content: `vault policy read file-encryptor-policy`
3. Test with Vault CLI using cert auth
4. Ensure policy allows datakey operations

### Data Lost After Restart

**Symptom**: Vault starts but transit key missing

**Explanation**: This is expected in dev mode. Data is in-memory only.

**Solutions**:
1. Run `02-configure-vault.sh` again to recreate configuration
2. For persistence, use file-based storage (see Production Considerations)

## Testing the Setup

### Manual Test: Full Workflow

```bash
# Terminal 1: Start Vault
cd scripts/vault-setup-enterprise
./01-start-vault-dev.sh

# Terminal 2: Configure Vault
cd scripts/vault-setup-enterprise
./02-configure-vault.sh

# Terminal 3: Start Vault Agent
vault agent -config=configs/vault-agent/vault-agent-enterprise-dev.hcl

# Terminal 4: Test file encryption
echo "Hello, Vault!" > test.txt

./bin/file-encryptor encrypt \
  -i test.txt \
  -o test.txt.enc \
  -c configs/examples/example-enterprise.hcl

# Verify files created
ls -la test.txt*
# Should see: test.txt.enc and test.txt.key

# Decrypt
./bin/file-encryptor decrypt \
  -i test.txt.enc \
  -k test.txt.key \
  -o decrypted.txt \
  -c configs/examples/example-enterprise.hcl

# Verify
diff test.txt decrypted.txt
# Should be identical
```

## Stopping and Cleaning Up

### Stop Services

```bash
# Stop Vault Agent
pkill -f "vault agent"

# Stop Vault server
pkill -f "vault server"
```

### Clean Up Test Files

```bash
# Remove test files
rm -f test.txt test.txt.enc test.txt.key decrypted.txt

# Remove certificates (to regenerate fresh)
cd scripts/test-certs
rm -f ca.crt ca-key.pem client.crt client-key.pem ca.srl
cd ../..
```

### Full Reset

To completely reset and start fresh:

```bash
# Stop all Vault processes
pkill vault

# Remove certificates
cd scripts/test-certs
rm -f *.crt *.pem *.srl
cd ../..

# Start over with step 1
cd scripts/vault-setup-enterprise
./01-start-vault-dev.sh
```

## Related Guides

- **HCP Vault Setup**: See [VAULT_SETUP_GUIDE.md](VAULT_SETUP_GUIDE.md)
- **CLI Usage**: See [CLI_MODE.md](CLI_MODE.md)
- **Architecture**: See [../ARCHITECTURE.md](../ARCHITECTURE.md)
- **Windows Users**: See [VAULT_ENTERPRISE_SETUP_GUIDE_WINDOWS.md](VAULT_ENTERPRISE_SETUP_GUIDE_WINDOWS.md)

## Additional Resources

- [Vault Dev Mode](https://developer.hashicorp.com/vault/docs/concepts/dev-server)
- [Vault Transit Engine](https://developer.hashicorp.com/vault/docs/secrets/transit)
- [Vault Agent](https://developer.hashicorp.com/vault/docs/agent)
- [Certificate Auth Method](https://developer.hashicorp.com/vault/docs/auth/cert)
- [Vault Production Hardening](https://developer.hashicorp.com/vault/tutorials/operations/production-hardening)
- [Envelope Encryption](https://developer.hashicorp.com/vault/tutorials/encryption-as-a-service/eaas-transit)
