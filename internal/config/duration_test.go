package config

import (
	"testing"
	"time"
)

func TestDuration_UnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{
			name:    "valid seconds",
			input:   "30s",
			want:    30 * time.Second,
			wantErr: false,
		},
		{
			name:    "valid minutes",
			input:   "5m",
			want:    5 * time.Minute,
			wantErr: false,
		},
		{
			name:    "valid hours",
			input:   "2h",
			want:    2 * time.Hour,
			wantErr: false,
		},
		{
			name:    "valid complex",
			input:   "1h30m",
			want:    90 * time.Minute,
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "zero duration",
			input:   "0s",
			wantErr: true,
		},
		{
			name:    "negative duration",
			input:   "-5s",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration
			err := d.UnmarshalText([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Errorf("UnmarshalText() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("UnmarshalText() unexpected error: %v", err)
				return
			}

			if d.Duration() != tt.want {
				t.Errorf("UnmarshalText() = %v, want %v", d.Duration(), tt.want)
			}
		})
	}
}

func TestDuration_Duration(t *testing.T) {
	d := Duration(30 * time.Second)
	if got := d.Duration(); got != 30*time.Second {
		t.Errorf("Duration() = %v, want %v", got, 30*time.Second)
	}
}

func TestDuration_String(t *testing.T) {
	d := Duration(30 * time.Second)
	if got := d.String(); got != "30s" {
		t.Errorf("String() = %v, want %v", got, "30s")
	}
}

func TestDuration_MarshalText(t *testing.T) {
	d := Duration(30 * time.Second)
	got, err := d.MarshalText()
	if err != nil {
		t.Errorf("MarshalText() unexpected error: %v", err)
	}
	if string(got) != "30s" {
		t.Errorf("MarshalText() = %v, want %v", string(got), "30s")
	}
}
