package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
		wantErr  bool
	}{
		// Valid inputs - humanize uses SI units (1kB = 1000 bytes, not 1024)
		{"1KB", "1KB", 1000, false},
		{"1MB", "1MB", 1000 * 1000, false},
		{"2MB", "2MB", 2 * 1000 * 1000, false},
		{"512KB", "512KB", 512 * 1000, false},
		{"10MB", "10MB", 10 * 1000 * 1000, false},
		{"1GB", "1GB", 1000 * 1000 * 1000, false},

		// With spaces
		{"1 MB", "1 MB", 1000 * 1000, false},
		{"2  MB", "2  MB", 2 * 1000 * 1000, false},

		// Different case
		{"1mb", "1mb", 1000 * 1000, false},
		{"1Mb", "1Mb", 1000 * 1000, false},
		{"1mB", "1mB", 1000 * 1000, false},

		// Short form
		{"1K", "1K", 1000, false},
		{"1M", "1M", 1000 * 1000, false},
		{"1G", "1G", 1000 * 1000 * 1000, false},

		// Bytes
		{"1024B", "1024B", 1024, false},
		{"4096B", "4096B", 4096, false},

		// Decimal numbers
		{"1.5MB", "1.5MB", int(1.5 * 1000 * 1000), false},
		{"0.5GB", "0.5GB", 500 * 1000 * 1000, false},

		// Edge cases for chunk size limits (64KB to 10MB)
		{"64KB", "64KB", 64 * 1000, false},
		{"10MB", "10MB", 10 * 1000 * 1000, false},

		// Plain number (humanize treats as bytes)
		{"plain number", "1024", 1024, false},

		// Invalid inputs
		{"empty", "", 0, true},
		{"invalid unit", "10XX", 0, true},
		{"invalid number", "abcKB", 0, true},
		{"negative", "-1MB", 0, true},
		{"just unit", "MB", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseSize(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{"bytes", 512, "512 B"},
		{"1KB", 1024, "1.0 kB"},                     // 1024/1000 = 1.024 → 1.0
		{"1MB", 1024 * 1024, "1.0 MB"},              // 1048576/1000000 = 1.048576 → 1.0
		{"2MB", 2 * 1024 * 1024, "2.1 MB"},          // 2097152/1000000 = 2.097152 → 2.1
		{"512KB", 512 * 1024, "524 kB"},             // 524288/1000 = 524.288 → 524
		{"10MB", 10 * 1024 * 1024, "10 MB"},         // 10485760/1000000 = 10.48576 → 10
		{"1GB", 1024 * 1024 * 1024, "1.1 GB"},       // 1073741824/1000000000 = 1.073741824 → 1.1
		{"1.5MB", int(1.5 * 1024 * 1024), "1.6 MB"}, // 1572864/1000000 = 1.572864 → 1.6
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseSizeRoundTrip(t *testing.T) {
	// Test that Parse -> Format -> Parse produces consistent results
	inputs := []string{"64KB", "512KB", "1MB", "2MB", "5MB", "10MB"}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			bytes, err := ParseSize(input)
			require.NoError(t, err)

			formatted := FormatSize(bytes)

			bytes2, err := ParseSize(formatted)
			require.NoError(t, err)

			assert.Equal(t, bytes, bytes2, "Round trip should produce same result")
		})
	}
}
