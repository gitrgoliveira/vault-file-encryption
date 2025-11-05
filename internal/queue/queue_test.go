package queue

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/gitrgoliveira/vault_file_encryption/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewQueue(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "queue-state.json")

	cfg := &Config{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   5 * time.Minute,
		StatePath:  statePath,
	}

	q, err := NewQueue(cfg)
	require.NoError(t, err)
	require.NotNil(t, q)
	assert.Equal(t, 3, q.maxRetries)
	assert.Equal(t, 1*time.Second, q.baseDelay)
	assert.Equal(t, 5*time.Minute, q.maxDelay)
}

func TestQueue_Enqueue(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "queue-state.json")

	cfg := &Config{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		StatePath:  statePath,
	}

	q, err := NewQueue(cfg)
	require.NoError(t, err)

	item := model.NewItem(model.OperationEncrypt, "/tmp/test.txt", "/tmp/test.enc")

	err = q.Enqueue(item)
	assert.NoError(t, err)
	assert.Equal(t, 1, q.Size())

	// Enqueue duplicate should fail
	err = q.Enqueue(item)
	assert.Error(t, err)
}

func TestQueue_Dequeue(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "queue-state.json")

	cfg := &Config{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		StatePath:  statePath,
	}

	q, err := NewQueue(cfg)
	require.NoError(t, err)

	// Empty queue should return nil
	item := q.Dequeue()
	assert.Nil(t, item)

	// Enqueue and dequeue
	item1 := model.NewItem(model.OperationEncrypt, "/tmp/test1.txt", "/tmp/test1.enc")
	item2 := model.NewItem(model.OperationEncrypt, "/tmp/test2.txt", "/tmp/test2.enc")

	err = q.Enqueue(item1)
	require.NoError(t, err)
	err = q.Enqueue(item2)
	require.NoError(t, err)

	// Should dequeue in FIFO order
	dequeued := q.Dequeue()
	assert.Equal(t, item1.ID, dequeued.ID)
	assert.Equal(t, 1, q.Size())

	dequeued = q.Dequeue()
	assert.Equal(t, item2.ID, dequeued.ID)
	assert.Equal(t, 0, q.Size())
}

func TestQueue_Requeue(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "queue-state.json")

	cfg := &Config{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   5 * time.Minute,
		StatePath:  statePath,
	}

	q, err := NewQueue(cfg)
	require.NoError(t, err)

	item := model.NewItem(model.OperationEncrypt, "/tmp/test.txt", "/tmp/test.enc")

	// Enqueue the item first
	err = q.Enqueue(item)
	require.NoError(t, err)

	// Dequeue it to simulate processing
	dequeued := q.Dequeue()
	require.NotNil(t, dequeued)
	dequeued.MarkProcessing()

	// Requeue with error
	err = q.Requeue(dequeued, assert.AnError)
	assert.NoError(t, err)
	assert.Equal(t, model.StatusFailed, dequeued.Status)
	assert.Equal(t, 1, q.Size())

	// Item should not be dequeued immediately (still in retry delay)
	item2 := q.Dequeue()
	assert.Nil(t, item2)

	// Wait for retry delay (baseDelay * 2^1 = 100ms * 2 = 200ms)
	time.Sleep(250 * time.Millisecond)

	// Now should be dequeued
	item3 := q.Dequeue()
	assert.NotNil(t, item3)
	assert.Equal(t, item.ID, item3.ID)
}

func TestQueue_RequeueDLQ(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "queue-state.json")

	cfg := &Config{
		MaxRetries: 2,
		BaseDelay:  1 * time.Second,
		StatePath:  statePath,
	}

	q, err := NewQueue(cfg)
	require.NoError(t, err)

	item := model.NewItem(model.OperationEncrypt, "/tmp/test.txt", "/tmp/test.enc")

	// First attempt
	item.MarkProcessing() // AttemptCount = 1
	err = q.Requeue(item, assert.AnError)
	assert.NoError(t, err) // 1 < 2, should succeed

	// Second attempt - should move to DLQ
	item.MarkProcessing() // AttemptCount = 2
	err = q.Requeue(item, assert.AnError)
	assert.Error(t, err) // 2 < 2 is false, should fail and move to DLQ
	assert.Equal(t, model.StatusDLQ, item.Status)
}

func TestQueue_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "queue-state.json")

	cfg := &Config{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		StatePath:  statePath,
	}

	q, err := NewQueue(cfg)
	require.NoError(t, err)

	// Add items
	item1 := model.NewItem(model.OperationEncrypt, "/tmp/test1.txt", "/tmp/test1.enc")
	item2 := model.NewItem(model.OperationEncrypt, "/tmp/test2.txt", "/tmp/test2.enc")

	err = q.Enqueue(item1)
	require.NoError(t, err)
	err = q.Enqueue(item2)
	require.NoError(t, err)

	// Save
	err = q.Save()
	assert.NoError(t, err)

	// Create new queue and load
	q2, err := NewQueue(cfg)
	require.NoError(t, err)

	err = q2.Load()
	assert.NoError(t, err)
	assert.Equal(t, 2, q2.Size())

	// Verify items
	items := q2.List()
	assert.Equal(t, item1.ID, items[0].ID)
	assert.Equal(t, item2.ID, items[1].ID)
}

func TestQueue_CalculateBackoff(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "queue-state.json")

	cfg := &Config{
		MaxRetries: 5,
		BaseDelay:  1 * time.Second,
		MaxDelay:   10 * time.Second,
		StatePath:  statePath,
	}

	q, err := NewQueue(cfg)
	require.NoError(t, err)

	tests := []struct {
		attempts int
		expected time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 10 * time.Second}, // Capped at maxDelay
		{5, 10 * time.Second}, // Capped at maxDelay
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempts_%d", tt.attempts), func(t *testing.T) {
			delay := q.calculateBackoff(tt.attempts)
			assert.Equal(t, tt.expected, delay)
		})
	}
}

func TestItem_ShouldRetry(t *testing.T) {
	tests := []struct {
		name         string
		attemptCount int
		maxRetries   int
		expected     bool
	}{
		{"within limit", 2, 3, true},
		{"at limit", 3, 3, false},
		{"over limit", 4, 3, false},
		{"infinite retries", 100, -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &model.Item{AttemptCount: tt.attemptCount}
			assert.Equal(t, tt.expected, item.ShouldRetry(tt.maxRetries))
		})
	}
}

func TestItem_MarkProcessing(t *testing.T) {
	item := model.NewItem(model.OperationEncrypt, "/tmp/test.txt", "/tmp/test.enc")
	assert.Equal(t, 0, item.AttemptCount)
	assert.Equal(t, model.StatusPending, item.Status)

	item.MarkProcessing()
	assert.Equal(t, 1, item.AttemptCount)
	assert.Equal(t, model.StatusProcessing, item.Status)
	assert.False(t, item.LastAttempt.IsZero())
}

func TestItem_MarkCompleted(t *testing.T) {
	item := model.NewItem(model.OperationEncrypt, "/tmp/test.txt", "/tmp/test.enc")
	item.MarkProcessing()
	item.Error = "some error"

	item.MarkCompleted()
	assert.Equal(t, model.StatusCompleted, item.Status)
	assert.Empty(t, item.Error)
	assert.False(t, item.CompletedAt.IsZero())
}

func TestItem_MarkFailed(t *testing.T) {
	item := model.NewItem(model.OperationEncrypt, "/tmp/test.txt", "/tmp/test.enc")
	item.MarkProcessing()

	item.MarkFailed(assert.AnError, 5*time.Second)
	assert.Equal(t, model.StatusFailed, item.Status)
	assert.NotEmpty(t, item.Error)
	assert.True(t, item.NextRetry.After(time.Now()))
}

func TestPersistence_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "test-state.json")

	p, err := NewPersistence(statePath)
	require.NoError(t, err)

	// Create items
	items := []*model.Item{
		model.NewItem(model.OperationEncrypt, "/tmp/test1.txt", "/tmp/test1.enc"),
		model.NewItem(model.OperationDecrypt, "/tmp/test2.enc", "/tmp/test2.txt"),
	}

	// Save
	err = p.Save(items)
	assert.NoError(t, err)
	assert.FileExists(t, statePath)

	// Load
	loaded, err := p.Load()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(loaded))
	assert.Equal(t, items[0].ID, loaded[0].ID)
	assert.Equal(t, items[1].ID, loaded[1].ID)
}

func TestPersistence_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "nonexistent.json")

	p, err := NewPersistence(statePath)
	require.NoError(t, err)

	// Should return empty list, not error
	items, err := p.Load()
	assert.NoError(t, err)
	assert.Empty(t, items)
}
