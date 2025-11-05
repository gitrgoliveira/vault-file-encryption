# Main Terraform configuration for Vault setup

# Enable Transit Secrets Engine
resource "vault_mount" "transit" {
  path        = var.transit_mount_path
  type        = "transit"
  description = "Transit engine for file encryption"

  options = {
    convergent_encryption = "false"
  }
}

# Enable Certificate Authentication
resource "vault_auth_backend" "cert" {
  type        = "cert"
  path        = var.cert_auth_mount_path
  description = "Certificate authentication for file-encryptor application"
}
