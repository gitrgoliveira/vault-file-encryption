# Transit Engine Configuration

# Create encryption key
resource "vault_transit_secret_backend_key" "file_encryption" {
  backend          = vault_mount.transit.path
  name             = var.transit_key_name
  type             = var.transit_key_type
  deletion_allowed = false
  exportable       = false
  
  # Enable key derivation for additional security
  derived = false
  
  # Auto-rotation disabled by default (can be enabled later)
  auto_rotate_period = 0
}

# Output key information
output "transit_key_name" {
  description = "Name of the created encryption key"
  value       = vault_transit_secret_backend_key.file_encryption.name
}

output "transit_key_type" {
  description = "Type of the encryption key"
  value       = vault_transit_secret_backend_key.file_encryption.type
}

output "transit_mount_path" {
  description = "Mount path of Transit engine"
  value       = vault_mount.transit.path
}
