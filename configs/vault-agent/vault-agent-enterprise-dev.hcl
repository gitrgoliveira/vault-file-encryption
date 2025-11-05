# Vault Agent Configuration for Local Vault Enterprise (Dev Mode)
# Uses certificate authentication

vault {
  address = "https://127.0.0.1:8200"
  # No namespace in dev mode (uses root namespace)
  
  # Dev mode uses self-signed certs - trust the generated CA
  ca_cert = "/Users/ricardo/repos/vault_file_encryption/scripts/vault-setup-enterprise/.vault-tls/vault-ca.pem"
}

# Auto-authentication using client certificate
auto_auth {
  method {
    type = "cert"
    
    config = {
      # Path to client certificate
      client_cert = "/Users/ricardo/repos/vault_file_encryption/scripts/test-certs/client.crt"
      
      # Path to client private key
      client_key = "/Users/ricardo/repos/vault_file_encryption/scripts/test-certs/client-key.pem"
      
      # Path to CA cert for verifying Vault server
      ca_cert = "/Users/ricardo/repos/vault_file_encryption/scripts/vault-setup-enterprise/.vault-tls/vault-ca.pem"
      
      # Certificate auth mount path
      mount_path = "auth/cert"
      
      # Role name (optional, uses cert CN if not specified)
      name = "file-encryptor"
    }
  }

  # Save authenticated token to file
  sink {
    type = "file"
    config = {
      path = "/tmp/vault-token-enterprise"
      mode = 0600
    }
  }
}

# API proxy - forward requests to Vault
api_proxy {
  use_auto_auth_token = true
}

# Listener for file-encryptor application
# Using port 8210 (different from HCP Vault Agent on 8200)
listener "tcp" {
  address = "127.0.0.1:8210"
  tls_disable = true
}

# Response caching
cache {
  # Cache consistency settings
  enforce_consistency = "always"
  when_inconsistent   = "retry"
}

# Logging
log_level = "info"
