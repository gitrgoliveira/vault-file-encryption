# Development Vault Agent Configuration
# Using token authentication instead of certificate auth for simplicity

vault {
  address = "https://vault-cluster-primary.vault.11eab575-aee3-cf27-adc9-0242ac11000a.aws.hashicorp.cloud:8200"
  namespace = "admin/vault_crypto"
}

# Auto-authentication using token (for development)
auto_auth {
  method {
    type = "token_file"
    
    config = {
      token_file_path = "/tmp/vault-token-dev"
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

cache {
  # Cache data key operations
  enforce_consistency = "always"
  when_inconsistent   = "retry"
}

# Development logging
log_level = "debug"
