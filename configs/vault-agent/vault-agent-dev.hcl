# Development Vault Agent Configuration

vault {
  address = "https://vault-cluster-public-vault-XXXXX.hashicorp.cloud:8200"
  namespace = "admin/vault_crypto"
}

auto_auth {
  method {
    type = "cert"
    
    config = {
      client_cert = "./scripts/test-certs/client.crt"
      client_key  = "./scripts/test-certs/client-key.pem"
      mount_path  = "auth/cert"
      name        = "file-encryptor"
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
