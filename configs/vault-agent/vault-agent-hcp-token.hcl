# Development Vault Agent Configuration - Token Auth
# For development use with existing Vault token

vault {
  address = "https://vault-cluster-primary.vault.11eab575-aee3-cf27-adc9-0242ac11000a.aws.hashicorp.cloud:8200"
  namespace = "admin/vault_crypto"
}

# Use the admin token directly for development
# In production, use proper authentication methods
auto_auth {
  method {
    type = "token"
    
    config = {
      # Token will be read from VAULT_TOKEN environment variable
    }
  }

  sink {
    type = "file"
    config = {
      path = "/tmp/vault-token"
      mode = 0600
    }
  }
}

api_proxy {
  use_auto_auth_token = true
}

listener "tcp" {
  address = "127.0.0.1:8200"
  tls_disable = true
}

cache {}

# Development logging
log_level = "info"
