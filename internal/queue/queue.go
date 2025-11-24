package queue

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/gitrgoliveira/vault-file-encryption/internal/config"
	"github.com/gitrgoliveira/vault-file-encryption/internal/model"
)

// Queue is a thread-safe FIFO queue with persistence
type Queue struct {
	mu    sync.RWMutex
	items *list.List

	// Map for quick lookup by ID
	itemMap map[string]*list.Element

	// Configuration
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration

	// Persistence
	persistence *Persistence
}

// Config holds queue configuration
type Config struct {
	MaxRetries int           // Maximum retry attempts (-1 for infinite)
	BaseDelay  time.Duration // Initial retry delay
	MaxDelay   time.Duration // Maximum retry delay
	StatePath  string        // Path to save queue state
}

// NewQueue creates a new FIFO queue
func NewQueue(cfg *Config) (*Queue, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if cfg.BaseDelay == 0 {
		cfg.BaseDelay = config.DefaultBaseDelay
	}
	if cfg.MaxDelay == 0 {
		cfg.MaxDelay = config.DefaultMaxDelay
	}

	persistence, err := NewPersistence(cfg.StatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create persistence: %w", err)
	}

	q := &Queue{
		items:       list.New(),
		itemMap:     make(map[string]*list.Element),
		maxRetries:  cfg.MaxRetries,
		baseDelay:   cfg.BaseDelay,
		maxDelay:    cfg.MaxDelay,
		persistence: persistence,
	}

	return q, nil
}

// Enqueue adds an item to the back of the queue
func (q *Queue) Enqueue(item *model.Item) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if item already exists
	if _, exists := q.itemMap[item.ID]; exists {
		return fmt.Errorf("item with ID %s already exists", item.ID)
	}

	// Add to queue
	element := q.items.PushBack(item)
	q.itemMap[item.ID] = element

	return nil
}

// Dequeue removes and returns the item from the front of the queue
// Returns nil if queue is empty or all items are not ready for retry
func (q *Queue) Dequeue() *model.Item {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()

	// Find first item that's ready to process
	for e := q.items.Front(); e != nil; e = e.Next() {
		item := e.Value.(*model.Item)

		// Skip items that are not ready for retry
		if item.Status == model.StatusFailed && now.Before(item.NextRetry) {
			continue
		}

		// Skip items in DLQ
		if item.Status == model.StatusDLQ {
			continue
		}

		// Remove from queue
		q.items.Remove(e)
		delete(q.itemMap, item.ID)

		return item
	}

	return nil
}

// Requeue adds a failed item back to the end of the queue
func (q *Queue) Requeue(item *model.Item, err error) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Calculate retry delay with exponential backoff
	delay := q.calculateBackoff(item.AttemptCount)
	item.MarkFailed(err, delay)

	// Check if item should be retried
	if !item.ShouldRetry(q.maxRetries) {
		item.MarkDLQ()
		// Don't add back to queue, but keep in itemMap for tracking
		return fmt.Errorf("item %s exceeded max retries, moved to DLQ", item.ID)
	}

	// Add back to end of queue
	element := q.items.PushBack(item)
	q.itemMap[item.ID] = element

	return nil
}

// Size returns the number of items in the queue
func (q *Queue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return q.items.Len()
}

// List returns all items in the queue
func (q *Queue) List() []*model.Item {
	q.mu.RLock()
	defer q.mu.RUnlock()

	items := make([]*model.Item, 0, q.items.Len())
	for e := q.items.Front(); e != nil; e = e.Next() {
		items = append(items, e.Value.(*model.Item))
	}

	return items
}

// Save persists the queue state to disk
func (q *Queue) Save() error {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return q.persistence.Save(q.List())
}

// Load restores the queue state from disk
func (q *Queue) Load() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	items, err := q.persistence.Load()
	if err != nil {
		return err
	}

	// Clear existing queue
	q.items = list.New()
	q.itemMap = make(map[string]*list.Element)

	// Add loaded items
	for _, item := range items {
		element := q.items.PushBack(item)
		q.itemMap[item.ID] = element
	}

	return nil
}

// calculateBackoff calculates exponential backoff delay using cenkalti/backoff library
func (q *Queue) calculateBackoff(attempts int) time.Duration {
	// For 0 attempts, return initial delay
	if attempts == 0 {
		return q.baseDelay
	}

	// Create exponential backoff with deterministic behavior
	b := &backoff.ExponentialBackOff{
		InitialInterval:     q.baseDelay,
		RandomizationFactor: 0,
		Multiplier:          2,
		MaxInterval:         q.maxDelay,
		MaxElapsedTime:      0, // Max retries handled separately
		Stop:                backoff.Stop,
		Clock:               backoff.SystemClock,
	}
	b.Reset()

	// Call NextBackOff() attempts+1 times to get delay for current attempt
	var delay time.Duration
	for i := 0; i <= attempts; i++ {
		delay = b.NextBackOff()
		if delay == backoff.Stop {
			return q.maxDelay
		}
	}

	return delay
}
