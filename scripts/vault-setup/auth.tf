# Certificate Authentication Configuration

# Read the CA certificate
data "local_file" "ca_cert" {
  filename = var.certificate_path
}

# Configure certificate authentication backend
resource "vault_cert_auth_backend_role" "file_encryptor" {
  name        = var.app_role_name
  backend     = vault_auth_backend.cert.path
  certificate = data.local_file.ca_cert.content
  
  # Allowed certificate common names
  allowed_common_names = var.allowed_common_names
  
  # Token configuration
  token_ttl             = 3600    # 1 hour
  token_max_ttl         = 86400   # 24 hours
  token_policies        = [vault_policy.file_encryptor.name]
  token_bound_cidrs     = []      # Add CIDR restrictions if needed
  token_explicit_max_ttl = 0
  token_no_default_policy = false
  token_num_uses         = 0      # Unlimited uses
  token_period           = 0
  token_type             = "default"
}

output "cert_auth_path" {
  description = "Path for certificate authentication"
  value       = vault_auth_backend.cert.path
}

output "cert_auth_role" {
  description = "Certificate authentication role name"
  value       = vault_cert_auth_backend_role.file_encryptor.name
}
