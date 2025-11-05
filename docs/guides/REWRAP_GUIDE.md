# Key Re-wrapping Guide

> **Platform Note**: This guide includes examples for both Unix/Linux/macOS (bash) and Windows (PowerShell). All features work identically on all platforms.

## Overview

Key re-wrapping is a security best practice that allows you to rotate encrypted data encryption keys (DEKs) to newer versions of your Vault Transit encryption key **without re-encrypting** your actual data files.

When you rotate your Vault Transit key, new encryptions use the latest version, but old encrypted DEKs remain at their original version. Re-wrapping updates these DEKs to the latest key version, improving your security posture.

## Key Concepts

### What Gets Updated
- **`.key` files**: Updated with new ciphertext using the latest Vault key version
- **`.enc` files**: Remain unchanged (no re-encryption needed)

### Why Re-wrap?
- Comply with key rotation policies
- Migrate to stronger encryption versions after key upgrades
- Maintain consistent key versions across your encrypted data estate

### How It Works
1. Read the encrypted DEK from the `.key` file
2. Send it to Vault Transit's `/rewrap` endpoint
3. Vault decrypts and re-encrypts the DEK with the latest key version
4. Atomically update the `.key` file with new ciphertext
5. Original `.enc` file remains valid and unchanged

## Command Reference

### Basic Usage

> **Windows users**: Replace `file-encryptor` with `file-encryptor.exe` and use backslashes or forward slashes for paths.

#### Re-wrap a Single Key File
```bash
# Unix/Linux/macOS
file-encryptor rewrap \
  --key-file /path/to/data.txt.key \
  --min-version 2
```

```powershell
# Windows
.\file-encryptor.exe rewrap `
  --key-file C:\path\to\data.txt.key `
  --min-version 2
```

#### Re-wrap All Keys in a Directory
```bash
# Unix/Linux/macOS - Non-recursive (current directory only)
file-encryptor rewrap \
  --dir /path/to/keys \
  --min-version 2

# Unix/Linux/macOS - Recursive (include subdirectories)
file-encryptor rewrap \
  --dir /path/to/keys \
  --recursive \
  --min-version 2
```

```powershell
# Windows - Non-recursive
.\file-encryptor.exe rewrap `
  --dir C:\path\to\keys `
  --min-version 2

# Windows - Recursive
.\file-encryptor.exe rewrap `
  --dir C:\path\to\keys `
  --recursive `
  --min-version 2
```

### Advanced Options

#### Dry-Run Mode
Preview what would be re-wrapped without making changes:
```bash
file-encryptor rewrap \
  --dir /path/to/keys \
  --recursive \
  --dry-run \
  --min-version 2
```

#### Disable Backups
By default, backups are created (`.key.bak`). To disable:
```bash
file-encryptor rewrap \
  --dir /path/to/keys \
  --min-version 2 \
  --backup=false
```

#### Output Formats

**Text (default)** - Human-readable summary:
```bash
file-encryptor rewrap --dir /path/to/keys --min-version 2
```
Output:
```
Re-wrap Summary:
  Total files: 50
  Successfully re-wrapped: 48
  Skipped (already at min version): 2
  Failed: 0

Version Distribution:
  Version 1: 15 files
  Version 2: 33 files
  Version 3: 2 files
```

**JSON** - Machine-readable with full details:
```bash
file-encryptor rewrap --dir /path/to/keys --min-version 2 --format json
```

**CSV** - Spreadsheet-compatible export:
```bash
file-encryptor rewrap --dir /path/to/keys --min-version 2 --format csv > rewrap-results.csv
```

### Flags Reference

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--key-file` | `-k` | Single key file to re-wrap | - |
| `--dir` | `-d` | Directory containing key files | - |
| `--recursive` | `-r` | Scan subdirectories recursively | `false` |
| `--dry-run` | - | Preview changes without modifying files | `false` |
| `--min-version` | `-m` | Minimum key version to require | `1` |
| `--backup` | `-b` | Create backups before re-wrapping | `true` |
| `--format` | `-f` | Output format: `text`, `json`, `csv` | `text` |
| `--config` | `-c` | Configuration file path | `config.hcl` |
| `--log-level` | `-l` | Log level: `debug`, `info`, `error` | `info` |

## Workflows

### 1. Audit Current Key Versions

Before re-wrapping, check current key version distribution using the `key-versions` command:
```bash
# Unix/Linux/macOS
# Display version statistics without making changes (no config needed)
file-encryptor key-versions \
  --dir /data/keys \
  --recursive

# Output as JSON for automated analysis
file-encryptor key-versions \
  --dir /data/keys \
  --recursive \
  --format json
```

```powershell
# Windows
# Display version statistics without making changes (no config needed)
.\file-encryptor.exe key-versions `
  --dir C:\data\keys `
  --recursive

# Output as JSON for automated analysis
.\file-encryptor.exe key-versions `
  --dir C:\data\keys `
  --recursive `
  --format json
```

**Note**: The `key-versions` command does not require a configuration file or Vault access - it only parses local `.key` files. This makes it faster and suitable for offline auditing.

### 2. Re-wrap with Safety Checks

Perform re-wrapping with backups enabled (default):
```bash
file-encryptor rewrap \
  --dir /data/keys \
  --recursive \
  --min-version 3 \
  --backup
```

Backups are created at `<key-file>.bak`. If re-wrapping fails, restore with:
```bash
cp /path/to/file.key.bak /path/to/file.key
```

### 3. Bulk Re-wrap for Compliance

Export results for audit trail:
```bash
file-encryptor rewrap \
  --dir /data/keys \
  --recursive \
  --min-version 4 \
  --format csv \
  > rewrap-audit-$(date +%Y%m%d).csv
```

CSV columns:
- `FilePath`: Path to the `.key` file
- `OldVersion`: Version before re-wrap
- `NewVersion`: Version after re-wrap
- `Status`: `success`, `skipped`, or `failed`
- `BackupCreated`: `true` or `false`
- `Error`: Error message if failed

### 4. Scheduled Re-wrapping

Create a cron job to regularly re-wrap keys:
```bash
# Daily re-wrap at 2 AM
0 2 * * * /usr/local/bin/file-encryptor rewrap \
  --dir /data/keys \
  --recursive \
  --min-version 2 \
  --format json \
  >> /var/log/rewrap-$(date +\%Y\%m).log 2>&1
```

## Best Practices

### 1. Always Test First
Use `--dry-run` before production re-wrapping:
```bash
file-encryptor rewrap --dir /data/keys --dry-run --min-version 3
```

### 2. Keep Backups Enabled
Unless you have a strong reason, keep backups enabled:
- Default `.bak` files allow quick rollback
- Backups are overwritten on subsequent re-wraps
- Remove backups manually after verification

### 3. Monitor Re-wrap Operations
Use structured logging for audit trails:
```bash
file-encryptor rewrap \
  --dir /data/keys \
  --recursive \
  --min-version 3 \
  --log-level info \
  --log-output /var/log/rewrap.log
```

### 4. Set Appropriate Min Version
- Check current Vault key version: `vault read transit/keys/file-encryption-key`
- Set `--min-version` to your organization's policy requirement
- Keys already at or above min version are skipped (no unnecessary Vault calls)

### 5. Handle Failures Gracefully
Re-wrap operations continue on individual file failures:
- Failed files are logged with error details
- Exit code `1` = partial success (some failures)
- Exit code `2` = complete failure (all files failed)
- Exit code `0` = complete success

## Troubleshooting

### Issue: "Key file already at minimum version"
**Symptom**: Files are skipped, no re-wrapping occurs

**Solution**: Check current key versions and increase `--min-version`:
```bash
# Check Vault key version
vault read transit/keys/file-encryption-key

# Adjust min-version accordingly
file-encryptor rewrap --dir /data/keys --min-version 4
```

### Issue: "Failed to rewrap: permission denied"
**Symptom**: Vault returns 403 errors

**Solution**: Ensure Vault policy allows `rewrap` capability:
```hcl
# Vault policy
path "transit/rewrap/file-encryption-key" {
  capabilities = ["update"]
}
```

### Issue: "File system permission denied"
**Symptom**: Cannot write updated `.key` file

**Solution**: Check file permissions:
```bash
# Ensure key files are writable
chmod 600 /path/to/*.key

# Ensure directory is writable (for temp files)
chmod 700 /path/to/keys
```

### Issue: "Dry-run shows changes but actual run skips files"
**Symptom**: Files reported as needing re-wrap in dry-run but skipped in actual run

**Solution**: Check if files were already re-wrapped between runs:
```bash
# Check actual key version
grep -oE 'vault:v[0-9]+' /path/to/file.key

# Compare with min-version parameter
file-encryptor rewrap --key-file /path/to/file.key --min-version 3
```

## Security Considerations

### 1. Atomic Updates
Re-wrap uses atomic file operations:
- Write to temporary file
- Sync to disk (`fsync`)
- Rename to target (atomic on POSIX)
- Rollback on failure using backup

### 2. Memory Safety
Plaintext DEKs are never stored during re-wrap:
- Vault handles decryption and re-encryption internally
- Only ciphertext is transmitted and stored
- No plaintext DEK touches disk or application memory

### 3. Audit Logging
Enable audit logging to track re-wrap operations:
```hcl
# config.hcl
audit_log {
  enabled = true
  path    = "/var/log/file-encryptor-audit.log"
}
```

## Performance

### Batch Performance
- **Small batches (<100 files)**: ~2-5 seconds
- **Medium batches (100-1000 files)**: ~30-60 seconds
- **Large batches (1000+ files)**: ~5-10 minutes

Performance factors:
- Vault network latency
- Number of Vault requests (1 per file)
- File system I/O for atomic writes
- Backup creation overhead

### Optimization Tips
1. **Use --backup=false** if you have external backups (saves I/O)
2. **Run during off-peak hours** to minimize Vault load
3. **Split large directories** for parallel processing:
   ```bash
   # Process different subdirectories in parallel
   file-encryptor rewrap --dir /data/keys/2024 --min-version 3 &
   file-encryptor rewrap --dir /data/keys/2023 --min-version 3 &
   wait
   ```

## Integration Examples

### CI/CD Pipeline
```yaml
# .github/workflows/key-rotation.yml
name: Monthly Key Re-wrap
on:
  schedule:
    - cron: '0 2 1 * *'  # First day of month at 2 AM

jobs:
  rewrap:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Re-wrap encryption keys
        env:
          VAULT_ADDR: ${{ secrets.VAULT_ADDR }}
          VAULT_TOKEN: ${{ secrets.VAULT_TOKEN }}
        run: |
          file-encryptor rewrap \
            --dir ./encrypted-keys \
            --recursive \
            --min-version 2 \
            --format json \
            > rewrap-results.json
      
      - name: Upload results
        uses: actions/upload-artifact@v3
        with:
          name: rewrap-results
          path: rewrap-results.json
```

### Monitoring Script
```bash
#!/bin/bash
# monitor-key-versions.sh

set -e

KEYS_DIR="/data/encrypted/keys"
MIN_VERSION=3
ALERT_THRESHOLD=10  # Alert if more than 10 files need rewrapping

# Dry-run to check what needs rewrapping
RESULT=$(file-encryptor rewrap \
  --dir "${KEYS_DIR}" \
  --recursive \
  --dry-run \
  --min-version "${MIN_VERSION}" \
  --format json)

# Parse JSON to count files needing rewrap
NEEDS_REWRAP=$(echo "${RESULT}" | jq '.successful')

if [ "${NEEDS_REWRAP}" -gt "${ALERT_THRESHOLD}" ]; then
  echo "WARNING: ${NEEDS_REWRAP} key files need re-wrapping to version ${MIN_VERSION}"
  # Send alert (e.g., Slack, PagerDuty, email)
  exit 1
fi

echo "OK: Only ${NEEDS_REWRAP} files need re-wrapping (threshold: ${ALERT_THRESHOLD})"
```

## See Also

- [CLI Mode Guide](CLI_MODE.md) - General CLI usage
- [Architecture Documentation](../ARCHITECTURE.md) - Technical details
- [Vault Setup Guide](VAULT_SETUP_GUIDE.md) - Vault configuration
- [Vault Enterprise Setup Guide](VAULT_ENTERPRISE_SETUP_GUIDE.md) - Enterprise-specific setup
