package sse

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// clientIDCounter is used to generate unique client IDs.
var clientIDCounter atomic.Int64

// client represents a connected SSE client.
type client struct {
	id         string
	events     chan Event
	filter     EventFilter
	ctx        context.Context
	cancel     context.CancelFunc
	lastActive time.Time
	closed     atomic.Bool
	closeMu    sync.Mutex
}

// newClient creates a new SSE client.
func newClient(ctx context.Context, bufferSize int, filter EventFilter) *client {
	clientCtx, cancel := context.WithCancel(ctx)

	return &client{
		id:         generateClientID(),
		events:     make(chan Event, bufferSize),
		filter:     filter,
		ctx:        clientCtx,
		cancel:     cancel,
		lastActive: time.Now(),
	}
}

// generateClientID creates a unique client identifier.
func generateClientID() string {
	id := clientIDCounter.Add(1)
	return fmt.Sprintf("sse-client-%d-%d", time.Now().UnixNano(), id)
}

// close terminates the client connection.
func (c *client) close() {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()

	if c.closed.Load() {
		return
	}

	c.closed.Store(true)
	c.cancel()
	close(c.events)
}

// isClosed returns true if the client has been closed.
func (c *client) isClosed() bool {
	return c.closed.Load()
}

// send attempts to send an event to the client.
// Returns false if the client buffer is full (slow client).
func (c *client) send(event Event) bool {
	if c.isClosed() {
		return false
	}

	// Apply filter if set
	if c.filter != nil && !c.filter(event) {
		return true // Event filtered out, but client is ok
	}

	select {
	case c.events <- event:
		c.lastActive = time.Now()
		return true
	default:
		// Buffer full - slow client
		return false
	}
}
