#!/bin/bash
# Start Vault Enterprise in dev mode
# This is for LOCAL DEVELOPMENT ONLY - data is in-memory and lost on restart

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Configuration
VAULT_ADDR="https://127.0.0.1:8200"
ROOT_TOKEN="dev-root-token"
TLS_CERT_DIR="${SCRIPT_DIR}/.vault-tls"

echo "[INFO] Starting Vault Enterprise in dev mode with TLS..."
echo "[INFO] Address: ${VAULT_ADDR}"
echo "[INFO] Root Token: ${ROOT_TOKEN}"
echo "[INFO] TLS Cert Directory: ${TLS_CERT_DIR}"
echo ""
echo "[WARNING] Dev mode stores data in-memory only!"
echo "[WARNING] All data will be lost when Vault stops."
echo ""

# Check if Vault is installed
if ! command -v vault &> /dev/null; then
    echo "[ERROR] Vault not found. Please install Vault first."
    echo "        Download from: https://developer.hashicorp.com/vault/downloads"
    exit 1
fi

# Show Vault version
echo "[INFO] Vault version:"
vault version
echo ""

# Check if Vault is already running
if lsof -Pi :8200 -sTCP:LISTEN -t >/dev/null 2>&1 ; then
    echo "[ERROR] Port 8200 is already in use."
    echo "        Stop existing Vault: pkill -f 'vault server'"
    echo "        Or check what's using port 8200: lsof -i :8200"
    exit 1
fi

# Create TLS cert directory
mkdir -p "${TLS_CERT_DIR}"
echo "[INFO] TLS certificates will be created in: ${TLS_CERT_DIR}"

# Save token to file for other scripts
echo "${ROOT_TOKEN}" > "${SCRIPT_DIR}/.vault-token"
chmod 600 "${SCRIPT_DIR}/.vault-token"
echo "[INFO] Root token saved to: ${SCRIPT_DIR}/.vault-token"
echo ""

# Start Vault in dev mode
echo "[INFO] Starting Vault server..."
echo "[INFO] Press Ctrl+C to stop the server"
echo ""
echo "=========================================="
echo "In another terminal, run:"
echo "  cd scripts/vault-setup-enterprise"
echo "  ./02-configure-vault.sh"
echo "=========================================="
echo ""

vault server -dev \
  -dev-root-token-id="${ROOT_TOKEN}" \
  -dev-listen-address="127.0.0.1:8200" \
  -dev-tls \
  -dev-tls-cert-dir="${TLS_CERT_DIR}"
