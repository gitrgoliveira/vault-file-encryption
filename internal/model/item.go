package model

import (
	"time"

	"github.com/google/uuid"
)

// OperationType defines the type of operation to be performed on a file.
type OperationType string

const (
	// OperationEncrypt represents an encryption operation.
	OperationEncrypt OperationType = "encrypt"
	// OperationDecrypt represents a decryption operation.
	OperationDecrypt OperationType = "decrypt"
)

// ItemStatus represents the status of a queue item
type ItemStatus string

const (
	StatusPending    ItemStatus = "pending"
	StatusProcessing ItemStatus = "processing"
	StatusCompleted  ItemStatus = "completed"
	StatusFailed     ItemStatus = "failed"
	StatusDLQ        ItemStatus = "dead_letter_queue"
)

// Item represents a work item in the processing queue.
type Item struct {
	// ID is a unique identifier for this item
	ID string `json:"id"`

	// Operation type: encrypt or decrypt
	Operation OperationType `json:"operation"`

	// SourcePath is the path to the source file
	SourcePath string `json:"source_path"`

	// DestPath is the path where the processed file should be saved
	DestPath string `json:"dest_path"`

	// KeyPath for the encryption key file
	KeyPath string `json:"key_path,omitempty"`

	// ChecksumPath for the checksum file
	ChecksumPath string `json:"checksum_path,omitempty"`

	// Status is the current status of this item
	Status ItemStatus `json:"status"`

	// AttemptCount is the number of times this item has been processed
	AttemptCount int `json:"attempt_count"`

	// LastAttempt is the timestamp of the last processing attempt
	LastAttempt time.Time `json:"last_attempt"`

	// NextRetry is when the next retry should occur
	NextRetry time.Time `json:"next_retry,omitempty"`

	// Error is the last error message
	Error string `json:"error,omitempty"`

	// CreatedAt is when this item was created
	CreatedAt time.Time `json:"created_at"`

	// CompletedAt is when this item was completed
	CompletedAt time.Time `json:"completed_at,omitempty"`

	// FileSize is the size of the source file in bytes
	FileSize int64 `json:"file_size"`

	// Checksum is the original file checksum
	Checksum string `json:"checksum,omitempty"`
}

// NewItem creates a new queue item.
func NewItem(op OperationType, source, dest string) *Item {
	return &Item{
		ID:           uuid.New().String(),
		Operation:    op,
		SourcePath:   source,
		DestPath:     dest,
		Status:       StatusPending,
		AttemptCount: 0,
		CreatedAt:    time.Now(),
	}
}

// ShouldRetry determines if the item should be retried based on the max retries limit.
func (i *Item) ShouldRetry(maxRetries int) bool {
	if maxRetries < 0 { // Infinite retries
		return true
	}
	return i.AttemptCount < maxRetries
}

// MarkProcessing marks the item as being processed
func (i *Item) MarkProcessing() {
	i.Status = StatusProcessing
	i.AttemptCount++
	i.LastAttempt = time.Now()
}

// MarkCompleted marks the item as completed
func (i *Item) MarkCompleted() {
	i.Status = StatusCompleted
	i.CompletedAt = time.Now()
	i.Error = ""
}

// MarkFailed updates the item's state after a failed processing attempt.
func (i *Item) MarkFailed(err error, retryDelay time.Duration) {
	i.Status = StatusFailed
	i.Error = err.Error()
	i.NextRetry = time.Now().Add(retryDelay)
}

// MarkDLQ moves the item to dead letter queue
func (i *Item) MarkDLQ() {
	i.Status = StatusDLQ
}
