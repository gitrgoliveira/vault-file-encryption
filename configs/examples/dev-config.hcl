# Development Configuration
# Simplified config for local development and testing
# Uses direct connection to HCP Vault (no Vault Agent required)

vault {
  # For development, use direct HCP Vault access
  # Set VAULT_TOKEN and VAULT_NAMESPACE in environment variables
  agent_address = "https://vault-cluster-primary.vault.11eab575-aee3-cf27-adc9-0242ac11000a.aws.hashicorp.cloud:8200"
  transit_mount = "transit"
  key_name = "file-encryption-key"
  request_timeout = "30s"
}

encryption {
  # Local test directories
  source_dir = "./test-data/source"
  dest_dir = "./test-data/encrypted"
  source_file_behavior = "archive"
  calculate_checksum = true
}

# Decryption configuration (optional)
decryption {
  enabled = true
  source_dir = "./test-data/encrypted"
  dest_dir = "./test-data/decrypted"
  source_file_behavior = "archive"
  verify_checksum = true
}

queue {
  state_path = "./queue-state.json"
  max_retries = 3
  base_delay = "1s"
  max_delay = "1m"
  stability_duration = "1s"
}

logging {
  level = "debug"
  output = "stdout"
  format = "text"
  audit_log = false
}

