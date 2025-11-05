# Vault File Encryption Configuration
# Complete example configuration with all available options

vault {
  # Vault Agent listener address (not the HCP Vault address)
  # In production, this points to the local Vault Agent sidecar
  agent_address = "http://127.0.0.1:8200"
  
  # Transit engine mount path
  transit_mount = "transit"
  
  # Encryption key name
  key_name = "file-encryption-key"
  
  # Request timeout (optional, default: 30s)
  request_timeout = "30s"
}

encryption {
  # Directory to watch for files to encrypt
  source_dir = "/data/source"
  
  # Directory where encrypted files will be stored
  dest_dir = "/data/encrypted"
  
  # What to do with source files after encryption: "archive", "delete", or "keep"
  source_file_behavior = "archive"
  
  # Calculate SHA256 checksum for source files (optional, default: false)
  calculate_checksum = true
  
  # Chunk size for encryption (optional, default: "1MB")
  # Valid range: 64KB to 10MB
  # Examples: "512KB", "2MB", "5MB"
  chunk_size = "1MB"
  
  # Optional: File pattern to match (glob pattern)
  # file_pattern = "*.txt"
}

# Decryption configuration (optional)
decryption {
  # Enable decryption mode
  enabled = true
  
  # Directory to watch for files to decrypt
  source_dir = "/data/encrypted"
  
  # Directory where decrypted files will be stored
  dest_dir = "/data/decrypted"
  
  # What to do with source files after decryption: "archive", "delete", or "keep"
  source_file_behavior = "archive"
  
  # Verify SHA256 checksum after decryption (optional, default: false)
  verify_checksum = true
}

queue {
  # Path to save queue state for persistence
  state_path = "/var/lib/file-encryptor/queue-state.json"
  
  # Maximum retry attempts (-1 for infinite, default: 3)
  max_retries = 3
  
  # Initial retry delay (default: 1s)
  base_delay = "1s"
  
  # Maximum retry delay (default: 5m)
  max_delay = "5m"
  
  # File stability duration - wait time before processing (default: 1s)
  stability_duration = "1s"
}

logging {
  # Log level: "debug", "info", or "error" (default: "info")
  level = "info"
  
  # Log output: "stdout", "stderr", or file path (default: "stdout")
  output = "/var/log/file-encryptor/app.log"
  
  # Log format: "text" or "json" (default: "text")
  format = "text"
  
  # Enable audit logging (default: false)
  audit_log = true
  
  # Audit log file path (required if audit_log is true)
  audit_path = "/var/log/file-encryptor/audit.log"
}

