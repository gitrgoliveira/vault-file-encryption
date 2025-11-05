package watcher

import (
	"context"
	"os"
	"time"
)

// PartialUploadDetector detects if a file is still being written
type PartialUploadDetector struct {
	stabilityDuration time.Duration
}

// NewPartialUploadDetector creates a new detector
func NewPartialUploadDetector(stabilityDuration time.Duration) *PartialUploadDetector {
	if stabilityDuration == 0 {
		stabilityDuration = 1 * time.Second
	}

	return &PartialUploadDetector{
		stabilityDuration: stabilityDuration,
	}
}

// IsStable checks if a file's size has been stable for the configured duration
func (d *PartialUploadDetector) IsStable(filePath string) (bool, error) {
	// Get initial file size and mod time
	info1, err := os.Stat(filePath)
	if err != nil {
		return false, err
	}
	size1 := info1.Size()
	modTime1 := info1.ModTime()

	// Wait for stability duration
	time.Sleep(d.stabilityDuration)

	// Get file size and mod time again
	info2, err := os.Stat(filePath)
	if err != nil {
		return false, err
	}
	size2 := info2.Size()
	modTime2 := info2.ModTime()

	// File is stable if both size and modification time haven't changed
	return size1 == size2 && modTime1.Equal(modTime2), nil
}

// WaitForStability waits until the file is stable with context support
func (d *PartialUploadDetector) WaitForStability(ctx context.Context, filePath string, maxWait time.Duration) error {
	start := time.Now()

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		stable, err := d.IsStable(filePath)
		if err != nil {
			return err
		}

		if stable {
			return nil
		}

		// Sleep between retry attempts to avoid CPU spinning
		time.Sleep(d.stabilityDuration)

		// Check if we've exceeded max wait time
		if maxWait > 0 && time.Since(start) > maxWait {
			return os.ErrDeadlineExceeded
		}
	}
}
