# Migration Guide: v1.x to v2.0.0

Version 2.0.0 introduces a major architectural change by integrating the `go-fileencrypt` library for the underlying encryption primitives. This results in a **breaking change** to the encrypted file format.

## Breaking Changes

### File Format Incompatibility
Files encrypted with versions prior to v2.0.0 are **NOT compatible** with v2.0.0+. You cannot decrypt old files with the new version.

**Action Required:**
Before upgrading to v2.0.0, you must:
1. Decrypt all existing files using your current version (v1.x).
2. Upgrade to v2.0.0.
3. Re-encrypt the files using v2.0.0.

### API Changes
If you are using this project as a library:
- `Encryptor` and `Decryptor` structs no longer expose internal buffer pools.
- `NewEncryptor` and `NewDecryptor` signatures remain the same, but internal behavior has changed.
- `EncryptFile` and `DecryptFile` methods now use `go-fileencrypt` options internally.

## New Features
- **Standardized File Format**: Uses `go-fileencrypt`'s secure, versioned file format with magic headers and salt.
- **Improved Security**: Leverages well-tested crypto primitives and memory safety features from `go-fileencrypt`.
- **Maintainability**: Core crypto logic is now offloaded to a dedicated library.

## Migration Steps

1. **Decrypt Existing Data (v1.x)**
   ```bash
   # Using v1.x binary
   vault-file-encryption decrypt --input my-secret.enc --output my-secret.txt --key my-secret.key
   ```

2. **Upgrade Binary**
   Download or build the v2.0.0 binary.

3. **Re-encrypt Data (v2.0.0)**
   ```bash
   # Using v2.0.0 binary
   vault-file-encryption encrypt --input my-secret.txt --output my-secret.enc
   ```
