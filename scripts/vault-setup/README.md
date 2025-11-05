# Vault Setup with Terraform

This directory contains Terraform configuration to set up HCP Vault for the file-encryptor application.

## Prerequisites

1. HCP Vault cluster running
2. Vault token with admin permissions
3. Terraform 1.5+ installed
4. CA certificate for client authentication

## Setup Steps

### 1. Generate Test Certificates (Development Only)

```bash
cd ../test-certs
./generate-certs.sh
cd ../vault-setup
```

### 2. Configure Variables

```bash
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your Vault details
```

### 3. Set Vault Token

```bash
export VAULT_TOKEN="your-admin-token"
export VAULT_ADDR="https://vault-cluster-public-vault-xxxxx.hashicorp.cloud:8200"
export VAULT_NAMESPACE="admin"
```

### 4. Initialize Terraform

```bash
terraform init
```

### 5. Plan and Apply

```bash
# Review the plan
terraform plan

# Apply configuration
terraform apply
```

### 6. Verify Setup

```bash
# List transit keys
vault list transit/keys

# Read key details
vault read transit/keys/file-encryption-key

# Test certificate authentication
vault login -method=cert \
  -client-cert=../test-certs/client.crt \
  -client-key=../test-certs/client-key.pem

# Test data key generation
vault write -f transit/datakey/plaintext/file-encryption-key
```

## Configuration Summary

After successful deployment:

```bash
terraform output configuration_summary
```

## Starting Vault Agent

```bash
vault agent -config=../../configs/vault-agent/vault-agent-dev.hcl
```

## Cleanup

```bash
terraform destroy
```

## Troubleshooting

### Certificate Authentication Fails

- Verify CA certificate uploaded correctly
- Check certificate common name matches allowed_common_names
- Ensure certificate is not expired
- Check Vault logs

### Transit Key Creation Fails

- Verify namespace is correct (admin for HCP Vault)
- Check permissions of Vault token
- Ensure transit engine is enabled

### Permission Denied

- Review policy in `policies.tf`
- Verify role is assigned correct policies
- Check token policies: `vault token lookup`

## Production Considerations

1. **Certificates**: Use organization's PKI
2. **Key Rotation**: Configure auto-rotation for transit keys
3. **Backup**: Enable Vault snapshots
4. **Monitoring**: Set up Vault audit logging
5. **High Availability**: Use HCP Vault's built-in HA
