# Vault Policy Definitions

# Policy for file-encryptor application
resource "vault_policy" "file_encryptor" {
  name = "${var.app_role_name}-policy"

  policy = <<EOT
# Allow generating data keys from the transit engine
path "${var.transit_mount_path}/datakey/plaintext/${var.transit_key_name}" {
  capabilities = ["create", "update"]
}

# Allow decrypting data keys
path "${var.transit_mount_path}/decrypt/${var.transit_key_name}" {
  capabilities = ["create", "update"]
}

# Allow reading key metadata (optional, for debugging)
path "${var.transit_mount_path}/keys/${var.transit_key_name}" {
  capabilities = ["read"]
}

# Deny direct encrypt/decrypt operations (we use envelope encryption)
path "${var.transit_mount_path}/encrypt/${var.transit_key_name}" {
  capabilities = ["deny"]
}

# Allow renewal of own token (if using tokens)
path "auth/token/renew-self" {
  capabilities = ["update"]
}

# Allow lookup of own token
path "auth/token/lookup-self" {
  capabilities = ["read"]
}
EOT
}

output "policy_name" {
  description = "Name of the created Vault policy"
  value       = vault_policy.file_encryptor.name
}
