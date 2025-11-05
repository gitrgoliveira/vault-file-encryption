package config

import (
	"encoding"
	"fmt"
	"time"
)

// Duration is a custom type that wraps time.Duration with HCL unmarshaling support.
// It implements encoding.TextUnmarshaler to enable automatic parsing from HCL string values.
type Duration time.Duration

// Ensure Duration implements encoding.TextUnmarshaler for HCL compatibility
var _ encoding.TextUnmarshaler = (*Duration)(nil)

// UnmarshalText implements encoding.TextUnmarshaler for HCL parsing.
// It parses duration strings like "30s", "5m", "1h" and validates they are positive.
func (d *Duration) UnmarshalText(text []byte) error {
	dur, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("invalid duration format: %w", err)
	}
	if dur <= 0 {
		return fmt.Errorf("duration must be positive, got: %v", dur)
	}
	*d = Duration(dur)
	return nil
}

// Duration returns the underlying time.Duration value.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// String returns the string representation of the duration.
func (d Duration) String() string {
	return time.Duration(d).String()
}

// MarshalText implements encoding.TextMarshaler for serialization.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}
