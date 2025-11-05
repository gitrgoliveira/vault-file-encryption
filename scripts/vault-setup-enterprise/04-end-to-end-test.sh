#!/bin/bash
# End-to-end test of Vault Enterprise setup
# Tests complete workflow: Vault -> Vault Agent -> File Encryption/Decryption

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Configuration
export VAULT_ADDR="https://127.0.0.1:8200"
export VAULT_SKIP_VERIFY=true

echo "=========================================="
echo "Vault Enterprise End-to-End Test"
echo "=========================================="
echo ""

# Test 1: Check Vault is running
echo "[TEST 1/6] Checking Vault server..."
if ! vault status > /dev/null 2>&1; then
    echo "[FAILED] Vault server is not running"
    echo "        Start it with: ./01-start-vault-dev.sh"
    exit 1
fi
echo "[OK] Vault server is running"
echo ""

# Test 2: Check Vault Agent is running
echo "[TEST 2/6] Checking Vault Agent..."
if ! lsof -Pi :8210 -sTCP:LISTEN -t >/dev/null 2>&1 ; then
    echo "[FAILED] Vault Agent is not running on port 8210"
    echo "        Start it with: vault agent -config=configs/vault-agent/vault-agent-enterprise-dev.hcl"
    exit 1
fi
echo "[OK] Vault Agent is running on port 8210"
echo ""

# Test 3: Check token file exists
echo "[TEST 3/6] Checking Vault Agent token..."
if [ ! -f /tmp/vault-token-enterprise ]; then
    echo "[FAILED] Token file not found at /tmp/vault-token-enterprise"
    exit 1
fi
echo "[OK] Vault Agent token file exists"
echo ""

# Test 4: Create test file
echo "[TEST 4/6] Creating test file..."
cd "${PROJECT_ROOT}"
mkdir -p test-data/source test-data/encrypted test-data/decrypted
TEST_FILE="test-data/source/e2e-test-$(date +%s).txt"
echo "This is an end-to-end test of Vault Enterprise integration!" > "${TEST_FILE}"
echo "[OK] Created test file: ${TEST_FILE}"
echo ""

# Test 5: Encrypt file
echo "[TEST 5/6] Encrypting file..."
ENCRYPTED_FILE="test-data/encrypted/$(basename ${TEST_FILE}).enc"
if ! ./bin/file-encryptor encrypt \
    -i "${TEST_FILE}" \
    -o "${ENCRYPTED_FILE}" \
    -c configs/examples/example-enterprise.hcl 2>&1 | grep -q "successfully"; then
    echo "[FAILED] Encryption failed"
    exit 1
fi

# Verify encrypted file and key exist
if [ ! -f "${ENCRYPTED_FILE}" ] || [ ! -f "${ENCRYPTED_FILE}.key" ]; then
    echo "[FAILED] Encrypted file or key not created"
    exit 1
fi
echo "[OK] File encrypted successfully"
echo "     Encrypted: ${ENCRYPTED_FILE}"
echo "     Key: ${ENCRYPTED_FILE}.key"
echo ""

# Test 6: Decrypt file
echo "[TEST 6/6] Decrypting file..."
DECRYPTED_FILE="test-data/decrypted/$(basename ${TEST_FILE})"
if ! ./bin/file-encryptor decrypt \
    -i "${ENCRYPTED_FILE}" \
    -k "${ENCRYPTED_FILE}.key" \
    -o "${DECRYPTED_FILE}" \
    -c configs/examples/example-enterprise.hcl 2>&1 | grep -q "successfully"; then
    echo "[FAILED] Decryption failed"
    exit 1
fi

# Verify decrypted file exists and content matches
if [ ! -f "${DECRYPTED_FILE}" ]; then
    echo "[FAILED] Decrypted file not created"
    exit 1
fi

ORIGINAL_CONTENT=$(cat "${TEST_FILE}")
DECRYPTED_CONTENT=$(cat "${DECRYPTED_FILE}")

if [ "${ORIGINAL_CONTENT}" != "${DECRYPTED_CONTENT}" ]; then
    echo "[FAILED] Decrypted content doesn't match original"
    echo "Original: ${ORIGINAL_CONTENT}"
    echo "Decrypted: ${DECRYPTED_CONTENT}"
    exit 1
fi
echo "[OK] File decrypted successfully"
echo "     Decrypted: ${DECRYPTED_FILE}"
echo "     Content matches original"
echo ""

# Cleanup
echo "Cleaning up test files..."
rm -f "${TEST_FILE}" "${ENCRYPTED_FILE}" "${ENCRYPTED_FILE}.key" "${DECRYPTED_FILE}"
echo ""

echo "=========================================="
echo "[OK] All end-to-end tests passed!"
echo "=========================================="
echo ""
echo "Your Vault Enterprise setup is fully functional:"
echo "  - Vault server running with TLS"
echo "  - Certificate authentication working"
echo "  - Vault Agent authenticating and proxying requests"
echo "  - File encryption/decryption working end-to-end"
echo ""
echo "You can now use the file-encryptor CLI with Vault Enterprise!"
