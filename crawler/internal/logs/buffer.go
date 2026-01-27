package logs

import (
	"bytes"
	"encoding/json"
	"sync"
	"time"
)

// circularBuffer implements Buffer as a thread-safe circular buffer.
type circularBuffer struct {
	entries   []LogEntry
	size      int
	head      int // Points to oldest entry
	count     int // Number of entries in buffer
	lineCount int // Total lines ever written
	mu        sync.RWMutex
}

// NewBuffer creates a new circular buffer with the specified capacity.
func NewBuffer(size int) Buffer {
	if size <= 0 {
		size = defaultBufferSize
	}
	return &circularBuffer{
		entries: make([]LogEntry, size),
		size:    size,
	}
}

// Write appends a log entry to the buffer.
func (b *circularBuffer) Write(entry LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Calculate the index for the new entry
	idx := (b.head + b.count) % b.size

	if b.count < b.size {
		// Buffer not full, just add
		b.entries[idx] = entry
		b.count++
	} else {
		// Buffer full, overwrite oldest
		b.entries[b.head] = entry
		b.head = (b.head + 1) % b.size
	}

	b.lineCount++
}

// ReadSince returns all entries since the given timestamp.
func (b *circularBuffer) ReadSince(since time.Time) []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]LogEntry, 0, b.count)
	for i := range b.count {
		idx := (b.head + i) % b.size
		if !b.entries[idx].Timestamp.Before(since) {
			result = append(result, b.entries[idx])
		}
	}
	return result
}

// ReadAll returns all buffered entries in chronological order.
func (b *circularBuffer) ReadAll() []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]LogEntry, b.count)
	for i := range b.count {
		idx := (b.head + i) % b.size
		result[i] = b.entries[idx]
	}
	return result
}

// Size returns the number of entries currently in the buffer.
func (b *circularBuffer) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}

// Clear empties the buffer.
func (b *circularBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.head = 0
	b.count = 0
	// Note: lineCount is not reset - it tracks total lines ever written
}

// Bytes returns the buffer content as JSON lines (for archiving).
func (b *circularBuffer) Bytes() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var buf bytes.Buffer
	for i := range b.count {
		idx := (b.head + i) % b.size
		line, marshalErr := json.Marshal(b.entries[idx])
		if marshalErr != nil {
			continue
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// LineCount returns the total number of lines ever written to the buffer.
func (b *circularBuffer) LineCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lineCount
}
