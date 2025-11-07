#!/bin/bash
# Configure Vault Enterprise for file encryption
# Sets up Transit engine, certificate authentication, and policies

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Configuration
VAULT_ADDR="https://127.0.0.1:8200"
ROOT_TOKEN=$(cat "${SCRIPT_DIR}/.vault-token" 2>/dev/null || echo "dev-root-token")

export VAULT_ADDR
export VAULT_SKIP_VERIFY=true  # Dev mode uses self-signed cert
export VAULT_TOKEN="${ROOT_TOKEN}"

echo "=========================================="
echo "Vault Enterprise Configuration"
echo "=========================================="
echo ""
echo "[INFO] Vault Address: ${VAULT_ADDR}"
echo "[INFO] Configuring Transit engine and certificate authentication..."
echo ""

# Wait for Vault to be ready
echo "[1/7] Waiting for Vault to be ready..."
for i in {1..10}; do
    if vault status &>/dev/null; then
        echo "[OK] Vault is ready"
        break
    fi
    if [ $i -eq 10 ]; then
        echo "[ERROR] Vault not ready after 10 seconds"
        echo "        Make sure Vault is running: ./01-start-vault-dev.sh"
        exit 1
    fi
    sleep 1
done
echo ""

# Enable Transit engine
echo "[2/7] Enabling Transit secrets engine..."
if vault secrets list | grep -q "^transit/"; then
    echo "[INFO] Transit engine already enabled"
else
    vault secrets enable -path=transit transit
    echo "[OK] Transit engine enabled at: transit/"
fi
echo ""

# Create encryption key
echo "[3/7] Creating encryption key..."
if vault list transit/keys 2>/dev/null | grep -q "file-encryption-key"; then
    echo "[INFO] Encryption key already exists"
else
    vault write -f transit/keys/file-encryption-key \
      type=aes256-gcm96 \
      deletion_allowed=false \
      exportable=false
    echo "[OK] Encryption key created: file-encryption-key"
fi
echo ""

# Generate test certificates
echo "[4/7] Generating test certificates..."
cd "${PROJECT_ROOT}/scripts/test-certs"
if [ ! -f "ca.crt" ] || [ ! -f "client.crt" ]; then
    echo "[INFO] Generating new certificates..."
    ./generate-certs.sh
else
    echo "[INFO] Certificates already exist, skipping generation"
    echo "       To regenerate: rm scripts/test-certs/*.{crt,pem,srl} && ./generate-certs.sh"
fi
cd "${SCRIPT_DIR}"
echo ""

# Enable certificate authentication
echo "[5/7] Enabling certificate authentication..."
if vault auth list | grep -q "^cert/"; then
    echo "[INFO] Certificate auth already enabled"
else
    vault auth enable cert
    echo "[OK] Certificate auth enabled at: auth/cert/"
fi
echo ""

# Upload CA certificate and configure role
echo "[6/7] Configuring certificate auth role..."
vault write auth/cert/certs/file-encryptor \
  certificate=@"${PROJECT_ROOT}/scripts/test-certs/ca.crt" \
  allowed_common_names="file-encryptor.example.local" \
  token_policies="file-encryptor-policy" \
  token_ttl=3600 \
  token_max_ttl=86400 \
  || echo "[INFO] Cert auth role may already exist"
echo "[OK] Certificate role configured: file-encryptor"
echo ""

# Create policy
echo "[7/7] Creating Vault policy..."
vault policy write file-encryptor-policy - <<EOF
# Allow generating plaintext data keys from Transit engine
path "transit/datakey/plaintext/file-encryption-key" {
  capabilities = ["create", "update"]
}

# Allow decrypting data keys
path "transit/decrypt/file-encryption-key" {
  capabilities = ["create", "update"]
}

# Allow reading key metadata (for debugging)
path "transit/keys/file-encryption-key" {
  capabilities = ["read"]
}

# Deny direct encrypt/decrypt (enforce envelope encryption pattern)
path "transit/encrypt/file-encryption-key" {
  capabilities = ["deny"]
}

# Allow token operations
path "auth/token/renew-self" {
  capabilities = ["update"]
}

path "auth/token/lookup-self" {
  capabilities = ["read"]
}
EOF
echo "[OK] Policy created: file-encryptor-policy"
echo ""

echo "=========================================="
echo "[OK] Vault configuration complete!"
echo "=========================================="
echo ""
echo "Summary:"
echo "  - Transit Engine: transit/"
echo "  - Encryption Key: file-encryption-key"
echo "  - Cert Auth Path: auth/cert/"
echo "  - Auth Role: file-encryptor"
echo "  - Policy: file-encryptor-policy"
echo ""
echo "Next steps:"
echo "  1. Test the setup:"
echo "     ./03-test-setup.sh"
echo ""
echo "  2. Start Vault Agent (from this directory):"
echo "     vault agent -config=../../configs/vault-agent/vault-agent-enterprise-dev.hcl"
echo ""
echo "  3. Use file-encryptor (from repo root):"
echo "     cd ../.."
echo "     ./bin/file-encryptor encrypt -i file.txt -o file.enc -c configs/examples/example-enterprise.hcl"
echo ""
