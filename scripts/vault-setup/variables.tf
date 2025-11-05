variable "vault_address" {
  description = "HCP Vault cluster address"
  type        = string
  default     = "https://vault-cluster-public-vault-xxxxx.hashicorp.cloud:8200"
}

variable "vault_namespace" {
  description = "Vault namespace (use 'admin' for HCP Vault)"
  type        = string
  default     = "admin"
}

variable "transit_mount_path" {
  description = "Mount path for Transit secrets engine"
  type        = string
  default     = "transit"
}

variable "transit_key_name" {
  description = "Name of the encryption key in Transit engine"
  type        = string
  default     = "file-encryption-key"
}

variable "transit_key_type" {
  description = "Type of encryption key (aes256-gcm96, aes128-gcm96, chacha20-poly1305, etc.)"
  type        = string
  default     = "aes256-gcm96"
}

variable "cert_auth_mount_path" {
  description = "Mount path for certificate authentication"
  type        = string
  default     = "cert"
}

variable "app_role_name" {
  description = "Name for the application role in Vault policies"
  type        = string
  default     = "file-encryptor"
}

variable "allowed_common_names" {
  description = "List of allowed certificate common names"
  type        = list(string)
  default     = ["file-encryptor.gitrgoliveira.local"]
}

variable "certificate_path" {
  description = "Path to the trusted CA certificate for cert auth"
  type        = string
  # This should be the CA that signed the client certificates
}
