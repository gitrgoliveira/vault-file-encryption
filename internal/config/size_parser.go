package config

import (
	"fmt"

	"github.com/dustin/go-humanize"
)

// ParseSize converts a size string (e.g., "2MB", "512KB", "10GB") to bytes.
// Uses dustin/go-humanize library for robust parsing with SI units (base 1000).
//
// Supported formats:
//   - Standard sizes: "64KB", "512KB", "1MB", "2MB", "10MB", "1GB"
//   - Decimal sizes: "1.5MB", "0.5GB"
//   - With spaces: "1 MB", "512 KB"
//   - Case insensitive: "1mb", "1MB", "1Mb"
//   - Short forms: "1K", "1M", "1G"
//   - Plain numbers: "1024" (treated as bytes)
//   - Byte suffix: "1024B", "4096B"
//
// Note: Uses SI units (base 1000) not binary units (base 1024):
//   - 1KB  = 1,000 bytes (not 1,024)
//   - 1MB  = 1,000,000 bytes (not 1,048,576)
//   - 1GB  = 1,000,000,000 bytes (not 1,073,741,824)
//
// For binary units, use IEC format: "1KiB" = 1,024 bytes
func ParseSize(sizeStr string) (int, error) {
	if sizeStr == "" {
		return 0, fmt.Errorf("size string cannot be empty")
	}

	// ParseBytes supports both SI (MB, KB) and IEC (MiB, KiB) formats
	bytes, err := humanize.ParseBytes(sizeStr)
	if err != nil {
		return 0, fmt.Errorf("invalid size format: %s (expected format: 2MB, 512KB, etc.): %w", sizeStr, err)
	}

	// Check for overflow when converting uint64 to int
	if bytes > uint64(int(^uint(0)>>1)) {
		return 0, fmt.Errorf("size too large: %s", sizeStr)
	}

	return int(bytes), nil
}

// FormatSize converts bytes to a human-readable string using SI units.
// Uses dustin/go-humanize library for consistent formatting.
//
// Output format examples:
//   - 512 B
//   - 1.0 kB
//   - 524 kB
//   - 2.1 MB
//   - 10 MB
//   - 1.0 GB
//
// Note: Outputs SI units (base 1000) with lowercase 'k' for kilobytes.
func FormatSize(bytes int) string {
	if bytes < 0 {
		return fmt.Sprintf("%dB", bytes)
	}
	return humanize.Bytes(uint64(bytes))
}
