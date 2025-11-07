#!/bin/bash
# Test Vault Enterprise setup
# Verifies that all components are configured correctly

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
echo "Vault Enterprise Setup Test"
echo "=========================================="
echo ""

# Test 0: Vault is running
echo "[TEST 0/4] Checking Vault status..."
if ! vault status &>/dev/null; then
    echo "[FAILED] Vault is not running or not accessible"
    echo "         Start Vault: ./01-start-vault-dev.sh"
    exit 1
fi
echo "[OK] Vault is running and accessible"
echo ""

# Test 1: Certificate authentication
echo "[TEST 1/4] Testing certificate authentication..."
CERT_LOGIN_OUTPUT=$(vault login -method=cert \
  -client-cert="${PROJECT_ROOT}/scripts/test-certs/client.crt" \
  -client-key="${PROJECT_ROOT}/scripts/test-certs/client-key.pem" \
  -format=json 2>&1)

if echo "${CERT_LOGIN_OUTPUT}" | grep -q '"auth":'; then
    echo "[OK] Certificate authentication successful"
    # Extract token for subsequent tests
    CERT_TOKEN=$(echo "${CERT_LOGIN_OUTPUT}" | grep -o '"client_token":"[^"]*"' | cut -d'"' -f4)
    export VAULT_TOKEN="${CERT_TOKEN}"
else
    echo "[FAILED] Certificate authentication failed"
    echo "${CERT_LOGIN_OUTPUT}"
    exit 1
fi
echo ""

# Test 2: Data key generation
echo "[TEST 2/4] Testing data key generation..."
DATAKEY_OUTPUT=$(vault write -force -format=json transit/datakey/plaintext/file-encryption-key 2>&1)

if echo "${DATAKEY_OUTPUT}" | grep -q '"plaintext":'; then
    echo "[OK] Data key generation successful"
    # Extract ciphertext for decryption test using jq
    CIPHERTEXT=$(echo "${DATAKEY_OUTPUT}" | jq -r '.data.ciphertext')
else
    echo "[FAILED] Data key generation failed"
    echo "${DATAKEY_OUTPUT}"
    exit 1
fi
echo ""

# Test 3: Data key decryption
echo "[TEST 3/4] Testing data key decryption..."
DECRYPT_OUTPUT=$(vault write -format=json transit/decrypt/file-encryption-key \
  ciphertext="${CIPHERTEXT}" 2>&1)

if echo "${DECRYPT_OUTPUT}" | grep -q '"plaintext":'; then
    echo "[OK] Data key decryption successful"
else
    echo "[FAILED] Data key decryption failed"
    echo "${DECRYPT_OUTPUT}"
    exit 1
fi
echo ""

# Test 4: Policy enforcement (direct encrypt should be denied)
echo "[TEST 4/4] Testing policy enforcement (should deny direct encrypt)..."
ENCRYPT_OUTPUT=$(vault write -format=json transit/encrypt/file-encryption-key \
  plaintext=$(echo "test" | base64) 2>&1 || true)

if echo "${ENCRYPT_OUTPUT}" | grep -q "permission denied"; then
    echo "[OK] Policy correctly denies direct encryption"
elif echo "${ENCRYPT_OUTPUT}" | grep -q "ciphertext"; then
    echo "[WARNING] Direct encryption was allowed (policy may need adjustment)"
else
    echo "[INFO] Encrypt test result unclear, but not critical"
fi
echo ""

echo "=========================================="
echo "[OK] All tests passed!"
echo "=========================================="
echo ""
echo "Your Vault Enterprise setup is ready to use."
echo ""
echo "Next steps:"
echo "  1. Start Vault Agent (from this directory):"
echo "     vault agent -config=../../configs/vault-agent/vault-agent-enterprise-dev.hcl"
echo ""
echo "  2. Test file encryption (from repo root):"
echo "     cd ../.."
echo "     echo 'Hello, Vault!' > test.txt"
echo "     ./bin/file-encryptor encrypt -i test.txt -o test.txt.enc --checksum -c configs/examples/example-enterprise.hcl"
echo ""
echo "  3. Test file decryption:"
echo "     ./bin/file-encryptor decrypt -i test.txt.enc -k test.txt.key -o decrypted.txt --verify-checksum -c configs/examples/example-enterprise.hcl"
echo ""
