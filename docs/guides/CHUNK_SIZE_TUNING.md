# Chunk Size Tuning Guide

## Overview

The chunk size determines how much data is encrypted/decrypted in a single AES-GCM operation. This guide helps you choose the optimal chunk size for your use case.

## Default Configuration

**Default Chunk Size:** 1 MB (1048576 bytes)

This default is optimized for general-purpose use and provides a good balance between:
- Memory usage
- CPU efficiency
- File I/O performance
- Progress reporting granularity

## Valid Range

- **Minimum:** 64 KB (65536 bytes)
- **Maximum:** 10 MB (10485760 bytes)

Values must be multiples of 4 KB (4096 bytes) for optimal AES alignment.

## Configuration Methods

### 1. Global Configuration (HCL)

Set the chunk size in your configuration file:

```hcl
encryption {
  chunk_size = "2MB"  # Apply to all operations
}
```

Supported formats:
- `"512KB"` - Kilobytes
- `"2MB"` - Megabytes  
- `"1048576"` - Bytes (no unit)

### 2. CLI Override

Override the configured chunk size for a specific operation:

```bash
# Encrypt with custom chunk size
./file-encryptor encrypt -i large.bin -o large.enc --chunk-size 4MB

# Decrypt uses the chunk size from when the file was encrypted
./file-encryptor decrypt -i large.enc -k large.key -o large.bin
```

**Note:** Decryption automatically reads the chunk size from the encrypted file header, so you don't need to specify it during decryption.

## Performance Characteristics

### Small Chunk Sizes (64KB - 512KB)

**Advantages:**
- Lower memory usage per operation
- More granular progress reporting
- Better for resource-constrained environments
- Faster cancellation response

**Disadvantages:**
- More CPU overhead (more encryption operations)
- More file I/O operations
- Higher metadata overhead
- Slower overall throughput

**Use When:**
- Running on systems with limited RAM
- Processing many small files sequentially
- Need responsive progress updates
- Files are typically small (<10MB)

### Medium Chunk Sizes (512KB - 2MB)

**Advantages:**
- Balanced memory/CPU trade-off
- Good throughput for most file sizes
- Reasonable progress granularity
- Minimal overhead

**Disadvantages:**
- Middle-ground trade-offs

**Use When:**
- General-purpose encryption (default)
- Mixed file sizes
- Standard server/workstation hardware
- Balanced requirements

### Large Chunk Sizes (2MB - 10MB)

**Advantages:**
- Maximum throughput for large files
- Fewer encryption operations
- Lower CPU overhead per byte
- Reduced metadata size

**Disadvantages:**
- Higher memory usage per operation
- Less granular progress reporting
- Slower cancellation response
- Larger buffer pool allocation

**Use When:**
- Processing very large files (>100MB)
- High-performance hardware available
- Throughput is critical
- Memory is not constrained

## Tuning Recommendations

### By File Size

| Average File Size | Recommended Chunk Size |
|------------------|------------------------|
| < 1 MB           | 256 KB                 |
| 1 MB - 10 MB     | 512 KB - 1 MB (default)|
| 10 MB - 100 MB   | 1 MB - 2 MB            |
| 100 MB - 1 GB    | 2 MB - 4 MB            |
| > 1 GB           | 4 MB - 8 MB            |

### By Use Case

**High-Throughput Batch Processing:**
```hcl
encryption {
  chunk_size = "4MB"  # Maximize throughput
}
```

**Real-Time Monitoring (Service Mode):**
```hcl
encryption {
  chunk_size = "1MB"  # Balanced default
}
```

**Resource-Constrained Environments:**
```hcl
encryption {
  chunk_size = "256KB"  # Minimize memory
}
```

**Docker/Container Deployments:**
```hcl
encryption {
  chunk_size = "512KB"  # Balance memory limits
}
```

## Memory Impact

Each encryption/decryption operation allocates buffers using a `sync.Pool`. Memory usage per operation:

```
Memory per operation ≈ chunk_size × 2 (read + write buffers)
```

**Examples:**
- 256 KB chunk → ~512 KB per operation
- 1 MB chunk → ~2 MB per operation
- 4 MB chunk → ~8 MB per operation

**Service Mode:**
The service processes files sequentially (one at a time), so memory usage is simply:

```
Total memory ≈ chunk_size × 2
```

For example, with the default 1 MB chunk size, expect approximately 2 MB of buffer memory during file processing.

**Note**: The service uses a single processor that handles one file at a time from the queue. This means chunk size affects the memory footprint for processing each individual file, not concurrent operations.

## File Format Compatibility

**Backward Compatibility:** Files encrypted with different chunk sizes can all be decrypted, as the chunk size is stored in the encrypted file format:

```
[12-byte nonce][4-byte chunk_size][encrypted_chunk]...
```

This means you can:
- Change chunk size between operations
- Decrypt files encrypted with any valid chunk size
- Mix files encrypted with different chunk sizes

**Important:** Once a file is encrypted, its chunk size is fixed. You cannot change it without re-encrypting the file.

## Monitoring and Validation

### Configuration Validation

The application validates chunk size on startup:

```bash
$ ./file-encryptor watch -c config.hcl
ERROR: invalid chunk_size: must be between 64KB and 10MB, got 16MB
```

### Progress Logging

Progress is logged at 20% intervals regardless of chunk size:

```
INFO Encryption progress file=large.bin progress=20.0
INFO Encryption progress file=large.bin progress=40.0
INFO Encryption progress file=large.bin progress=60.0
INFO Encryption progress file=large.bin progress=80.0
INFO Encryption complete file=large.bin
```

With larger chunk sizes, you'll see fewer progress updates for small files.

## Troubleshooting

### Out of Memory Errors

**Symptom:** Application crashes with OOM errors during encryption.

**Solution:** Reduce chunk size:
```hcl
encryption {
  chunk_size = "256KB"  # Down from 4MB
}
```

### Slow Performance

**Symptom:** Encryption is slower than expected for large files.

**Solution:** Increase chunk size if memory allows:
```hcl
encryption {
  chunk_size = "4MB"  # Up from 1MB
}
```

### Validation Errors

**Symptom:** 
```
ERROR: invalid chunk_size: must be a multiple of 4096 bytes
```

**Solution:** Ensure chunk size is AES-aligned:
```hcl
encryption {
  chunk_size = "512KB"  # OK - 524288 is divisible by 4096
  # chunk_size = "500KB"  # FAIL - 512000 not divisible by 4096
}
```

## Benchmarking

To find the optimal chunk size for your environment, use the CLI encrypt command with different sizes:

```bash
# Test 512KB chunks
time ./file-encryptor encrypt -i test.bin -o test1.enc --chunk-size 512KB

# Test 2MB chunks
time ./file-encryptor encrypt -i test.bin -o test2.enc --chunk-size 2MB

# Test 4MB chunks
time ./file-encryptor encrypt -i test.bin -o test3.enc --chunk-size 4MB
```

Compare:
- Total execution time
- Memory usage (use `top` or `htop` during execution)
- CPU utilization

## Best Practices

1. **Start with the default (1MB)** - Good for most use cases
2. **Profile before tuning** - Measure actual performance before changing
3. **Consider memory limits** - Especially in containerized environments
4. **Test with representative files** - Use actual file sizes from your workload
5. **Monitor in production** - Watch for OOM errors or performance issues
6. **Document your choice** - Add comments to your HCL config explaining the chunk size

## Example Configurations

### Cloud Storage Backup
```hcl
# Large files, high throughput required
encryption {
  chunk_size = "4MB"
}
```

### IoT/Edge Device
```hcl
# Limited memory, small files
encryption {
  chunk_size = "128KB"
}
```

### General Server
```hcl
# Balanced default
encryption {
  chunk_size = "1MB"
}
```

### High-Security Archive
```hcl
# Optimized for verification speed
encryption {
  chunk_size = "2MB"
  generate_checksum = true
}
```
