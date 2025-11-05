package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPartialUploadDetector(t *testing.T) {
	// Default duration
	detector := NewPartialUploadDetector(0)
	assert.Equal(t, 1*time.Second, detector.stabilityDuration)

	// Custom duration
	detector = NewPartialUploadDetector(2 * time.Second)
	assert.Equal(t, 2*time.Second, detector.stabilityDuration)
}

func TestPartialUploadDetector_IsStable(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create a file
	err := os.WriteFile(testFile, []byte("initial content"), 0644)
	require.NoError(t, err)

	detector := NewPartialUploadDetector(100 * time.Millisecond)

	// File should be stable (no changes)
	stable, err := detector.IsStable(testFile)
	assert.NoError(t, err)
	assert.True(t, stable)
}

func TestPartialUploadDetector_IsStable_Changing(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create a file
	err := os.WriteFile(testFile, []byte("initial"), 0644)
	require.NoError(t, err)

	detector := NewPartialUploadDetector(100 * time.Millisecond)

	// Start a goroutine that modifies the file
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.WriteFile(testFile, []byte("modified content"), 0644)
	}()

	// File should not be stable (changing)
	stable, err := detector.IsStable(testFile)
	assert.NoError(t, err)
	assert.False(t, stable)
}

func TestPartialUploadDetector_WaitForStability(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create a file
	err := os.WriteFile(testFile, []byte("initial content"), 0644)
	require.NoError(t, err)

	detector := NewPartialUploadDetector(100 * time.Millisecond)

	// File should become stable quickly
	ctx := context.Background()
	err = detector.WaitForStability(ctx, testFile, 5*time.Second)
	assert.NoError(t, err)
}

func TestPartialUploadDetector_IsStable_NonExistent(t *testing.T) {
	detector := NewPartialUploadDetector(100 * time.Millisecond)

	// Non-existent file should return error
	stable, err := detector.IsStable("/nonexistent/file.txt")
	assert.Error(t, err)
	assert.False(t, stable)
}

func TestPartialUploadDetector_WaitForStability_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create a file
	err := os.WriteFile(testFile, []byte("initial"), 0644)
	require.NoError(t, err)

	detector := NewPartialUploadDetector(100 * time.Millisecond)

	// Start a goroutine that keeps modifying the file
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 10; i++ {
			time.Sleep(80 * time.Millisecond)
			_ = os.WriteFile(testFile, []byte("modified"), 0644)
		}
	}()

	// Wait for stability with a short timeout - should timeout
	ctx := context.Background()
	err = detector.WaitForStability(ctx, testFile, 300*time.Millisecond)
	assert.Error(t, err)
	assert.ErrorIs(t, err, os.ErrDeadlineExceeded)

	<-done // Wait for goroutine to finish
}

func TestPartialUploadDetector_WaitForStability_ContextCancelled(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create a file
	err := os.WriteFile(testFile, []byte("initial"), 0644)
	require.NoError(t, err)

	// Use a longer stability duration to ensure context cancellation happens first
	detector := NewPartialUploadDetector(500 * time.Millisecond)

	// Create a context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Start stability check in goroutine
	errChan := make(chan error, 1)
	go func() {
		// Keep modifying the file to prevent it from becoming stable
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = os.WriteFile(testFile, []byte("modified"), 0644)
			}
		}
	}()

	go func() {
		errChan <- detector.WaitForStability(ctx, testFile, 10*time.Second)
	}()

	// Cancel the context after a short delay
	time.Sleep(150 * time.Millisecond)
	cancel()

	// Should return context.Canceled error
	err = <-errChan
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestPartialUploadDetector_WaitForStability_FileDeleted(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create a file
	err := os.WriteFile(testFile, []byte("initial"), 0644)
	require.NoError(t, err)

	// Use a longer stability duration
	detector := NewPartialUploadDetector(500 * time.Millisecond)

	// Keep file changing, then delete it
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Modify file a few times to keep it unstable
		for i := 0; i < 3; i++ {
			time.Sleep(100 * time.Millisecond)
			_ = os.WriteFile(testFile, []byte("modified"), 0644)
		}
		// Then delete it
		time.Sleep(100 * time.Millisecond)
		_ = os.Remove(testFile)
	}()

	// Wait for stability - should error when file is deleted
	ctx := context.Background()
	err = detector.WaitForStability(ctx, testFile, 5*time.Second)
	assert.Error(t, err)

	<-done // Wait for goroutine to finish
}
