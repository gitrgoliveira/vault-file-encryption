# Terraform Outputs

output "vault_address" {
  description = "Vault cluster address"
  value       = var.vault_address
}

output "vault_namespace" {
  description = "Vault namespace"
  value       = var.vault_namespace
}

output "configuration_summary" {
  description = "Summary of Vault configuration"
  value = {
    transit_path     = vault_mount.transit.path
    transit_key      = vault_transit_secret_backend_key.file_encryption.name
    cert_auth_path   = vault_auth_backend.cert.path
    cert_auth_role   = vault_cert_auth_backend_role.file_encryptor.name
    policy_name      = vault_policy.file_encryptor.name
  }
}
