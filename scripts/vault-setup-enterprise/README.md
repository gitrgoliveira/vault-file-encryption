# Vault Enterprise Setup Scripts - Dev Mode

This directory contains scripts to quickly set up a local Vault Enterprise instance in dev mode for testing the file-encryptor application.

## Quick Start

```bash
# Terminal 1: Start Vault with TLS
./01-start-vault-dev.sh

# Terminal 2: Configure Vault
./02-configure-vault.sh

# Terminal 3: Test setup
./03-test-setup.sh

# Terminal 4: Run end-to-end test
./04-end-to-end-test.sh
```

## Scripts

### 01-start-vault-dev.sh
Starts Vault Enterprise in dev mode on `https://127.0.0.1:8200` with TLS enabled.

**Features**:
- Auto-unsealed
- In-memory storage (data lost on restart)
- Root token: `dev-root-token`
- TLS enabled with self-signed certificates
- Certificates saved to `.vault-tls/` directory

**Usage**:
```bash
./01-start-vault-dev.sh
```

Leave this running in a terminal. Press Ctrl+C to stop.

### 02-configure-vault.sh
Configures Vault with Transit engine, encryption key, certificate auth, and policies.

**What it does**:
1. Enables Transit secrets engine at `transit/`
2. Creates encryption key `file-encryption-key` (AES-256-GCM)
3. Generates test certificates (in `scripts/test-certs/`)
4. Configures certificate authentication at `auth/cert/`
5. Creates `file-encryptor-policy` with appropriate permissions

**Usage**:
```bash
# In a new terminal, after starting Vault
./02-configure-vault.sh
```

### 03-test-setup.sh
Verifies the Vault setup is working correctly with TLS and certificate authentication.

**Tests**:
1. Vault is accessible (HTTPS)
2. Certificate authentication works
3. Data key generation succeeds
4. Data key decryption succeeds
5. Policy enforcement (denies direct encrypt)

**Usage**:
```bash
./03-test-setup.sh
```

### 04-end-to-end-test.sh
Complete integration test of the entire workflow.

**Tests**:
1. Vault server is running
2. Vault Agent is running on port 8210
3. Vault Agent token file exists
4. File encryption via Vault Agent
5. File decryption via Vault Agent
6. Content verification

**Usage**:
```bash
# Make sure Vault and Vault Agent are running first
./04-end-to-end-test.sh
```

## Environment Variables

See `vault-dev.env.example` for environment variable reference.

For most use cases, you don't need to set any environment variables. The scripts handle everything automatically.

## Important Notes

### Dev Mode Limitations
- **In-memory storage**: All data is lost when Vault stops
- **Self-signed TLS**: Uses auto-generated certificates (not CA-signed)
- **Not for production**: Security is relaxed for development

### TLS Configuration
- Vault runs with TLS enabled (`-dev-tls` flag)
- Certificates auto-generated in `.vault-tls/` directory
- `vault-ca.pem` - CA certificate for verifying server
- `vault-cert.pem` - Server certificate
- `vault-key.pem` - Server private key
- Vault Agent must use `ca_cert` or `tls_skip_verify` to connect

### Data Persistence
If you restart Vault, you'll need to run `02-configure-vault.sh` again to recreate the configuration.

### Port Conflicts
If port 8200 is in use, either:
- Stop the conflicting service
- Modify the scripts to use a different port

## Troubleshooting

### "Port 8200 already in use"
```bash
# Check what's using the port
lsof -i :8200

# Kill Vault processes
pkill -f "vault server"
```

### "Vault not ready"
Make sure `01-start-vault-dev.sh` is running in another terminal.

### Certificate errors
Regenerate certificates:
```bash
cd ../test-certs
rm -f *.crt *.pem *.srl
./generate-certs.sh
```

Then run `02-configure-vault.sh` again.

### Start fresh
```bash
# Stop Vault
pkill -f "vault server"

# Remove tokens and start over
rm .vault-token
./01-start-vault-dev.sh
```

## Next Steps

After successful setup:

1. **Start Vault Agent** (from `scripts/vault-setup-enterprise/`):
   ```bash
   vault agent -config=../../configs/vault-agent/vault-agent-enterprise-dev.hcl
   ```

2. **Use file-encryptor** (from repo root):
   ```bash
   cd ../..
   
   # Encrypt
   ./bin/file-encryptor encrypt \
     -i test.txt \
     -o test.enc \
     -c configs/examples/example-enterprise.hcl
   
   # Decrypt
   ./bin/file-encryptor decrypt \
     -i test.enc \
     -k test.key \
     -o decrypted.txt \
     -c configs/examples/example-enterprise.hcl
   ```

## Complete Guide

For detailed documentation, see: [docs/guides/VAULT_ENTERPRISE_SETUP_GUIDE.md](../../docs/guides/VAULT_ENTERPRISE_SETUP_GUIDE.md)

## Production Deployment

These scripts are for **development only**. For production Vault Enterprise deployment, see:
- [Vault Production Hardening](https://developer.hashicorp.com/vault/tutorials/operations/production-hardening)
- [Vault Enterprise Documentation](https://developer.hashicorp.com/vault/docs/enterprise)
