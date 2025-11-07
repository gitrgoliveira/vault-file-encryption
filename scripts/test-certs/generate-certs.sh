#!/bin/bash
set -e

# Generate test certificates for development
# DO NOT USE THESE IN PRODUCTION

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERTS_DIR="${SCRIPT_DIR}"

echo "Generating test certificates for Vault certificate authentication..."

# Generate CA private key
openssl genrsa -out "${CERTS_DIR}/ca-key.pem" 4096

# Generate CA certificate
openssl req -new -x509 -days 3650 -key "${CERTS_DIR}/ca-key.pem" \
  -out "${CERTS_DIR}/ca.crt" \
  -subj "/C=US/ST=State/L=City/O=TestOrg/OU=Development/CN=TestOrg-CA"

# Generate client private key
openssl genrsa -out "${CERTS_DIR}/client-key.pem" 4096

# Generate client certificate signing request
openssl req -new -key "${CERTS_DIR}/client-key.pem" \
  -out "${CERTS_DIR}/client.csr" \
  -subj "/C=US/ST=State/L=City/O=TestOrg/OU=FileEncryptor/CN=file-encryptor.example.local"

# Sign client certificate with CA
openssl x509 -req -days 3650 -in "${CERTS_DIR}/client.csr" \
  -CA "${CERTS_DIR}/ca.crt" -CAkey "${CERTS_DIR}/ca-key.pem" \
  -CAcreateserial -out "${CERTS_DIR}/client.crt"

# Clean up CSR
rm "${CERTS_DIR}/client.csr"

echo "✓ CA certificate: ${CERTS_DIR}/ca.crt"
echo "✓ Client certificate: ${CERTS_DIR}/client.crt"
echo "✓ Client key: ${CERTS_DIR}/client-key.pem"
echo ""
echo "⚠️  These are TEST certificates only. Use proper PKI in production."
