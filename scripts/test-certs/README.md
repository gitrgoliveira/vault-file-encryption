# Test Certificates

This directory contains scripts to generate test certificates for development.

## Generate Test Certificates

```bash
chmod +x generate-certs.sh
./generate-certs.sh
```

This creates:
- `ca.crt` - CA certificate (upload to Vault)
- `ca-key.pem` - CA private key (keep secure)
- `client.crt` - Client certificate (for file-encryptor)
- `client-key.pem` - Client private key (for file-encryptor)

## Usage

1. Generate certificates: `./generate-certs.sh`
2. Upload `ca.crt` to Vault when running Terraform
3. Configure Vault Agent with `client.crt` and `client-key.pem`

## Security Warning

These are self-signed certificates for **development only**.

In production:
- Use your organization's PKI
- Follow certificate management policies
- Implement proper key rotation
- Use HSM for CA key storage
