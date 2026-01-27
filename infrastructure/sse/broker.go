package sse

import (
	"context"
	"fmt"
	"sync"
	"time"

	infralogger "github.com/north-cloud/infrastructure/logger"
)

// broker implements the Broker interface.
type broker struct {
	logger  infralogger.Logger
	clients map[string]*client
	mu      sync.RWMutex

	// Event distribution
	publish chan Event

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Configuration
	eventBufferSize   int
	clientBufferSize  int
	heartbeatInterval time.Duration
	shutdownTimeout   time.Duration
	maxClients        int
}

// NewBroker creates a new SSE broker.
func NewBroker(logger infralogger.Logger, opts ...BrokerOption) Broker {
	b := &broker{
		logger:            logger,
		clients:           make(map[string]*client),
		eventBufferSize:   DefaultEventBufferSize,
		clientBufferSize:  DefaultClientBufferSize,
		heartbeatInterval: DefaultHeartbeatInterval,
		shutdownTimeout:   DefaultShutdownTimeout,
		maxClients:        DefaultMaxClients,
	}

	for _, opt := range opts {
		opt(b)
	}

	// Create publish channel with configured size
	b.publish = make(chan Event, b.eventBufferSize)

	return b
}

// Start begins processing events.
func (b *broker) Start(ctx context.Context) error {
	b.ctx, b.cancel = context.WithCancel(ctx)

	b.wg.Add(1)
	go b.broadcastLoop()

	b.logger.Info("SSE broker started",
		infralogger.Int("event_buffer_size", b.eventBufferSize),
		infralogger.Int("client_buffer_size", b.clientBufferSize),
		infralogger.Duration("heartbeat_interval", b.heartbeatInterval),
		infralogger.Int("max_clients", b.maxClients),
	)

	return nil
}

// Stop gracefully shuts down the broker.
func (b *broker) Stop() error {
	if b.cancel != nil {
		b.cancel()
	}

	// Wait for broadcast loop to finish with timeout
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		b.logger.Info("SSE broker stopped gracefully")
	case <-time.After(b.shutdownTimeout):
		b.logger.Warn("SSE broker shutdown timeout exceeded")
	}

	return nil
}

// Publish sends an event to all connected clients.
func (b *broker) Publish(ctx context.Context, event Event) error {
	select {
	case b.publish <- event:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("publish cancelled: %w", ctx.Err())
	default:
		return fmt.Errorf("publish buffer full (dropped event: %s)", event.Type)
	}
}

// Subscribe creates a new SSE subscription.
func (b *broker) Subscribe(ctx context.Context, opts ...ClientOption) (events <-chan Event, cleanup func()) {
	clientOpts := ClientOptions{
		BufferSize: b.clientBufferSize,
	}

	for _, opt := range opts {
		opt(&clientOpts)
	}

	// Check max clients
	b.mu.RLock()
	currentClients := len(b.clients)
	b.mu.RUnlock()

	if b.maxClients > 0 && currentClients >= b.maxClients {
		b.logger.Warn("Max SSE clients reached, rejecting new connection",
			infralogger.Int("max_clients", b.maxClients),
			infralogger.Int("current_clients", currentClients),
		)
		// Return a closed channel to signal rejection
		closed := make(chan Event)
		close(closed)
		return closed, func() {}
	}

	c := newClient(ctx, clientOpts.BufferSize, clientOpts.Filter)

	b.mu.Lock()
	b.clients[c.id] = c
	b.mu.Unlock()

	b.logger.Debug("Client subscribed",
		infralogger.String("client_id", c.id),
		infralogger.Int("total_clients", b.ClientCount()),
	)

	// Start cleanup goroutine
	b.wg.Add(1)
	go b.cleanupClient(c)

	// Return cleanup function
	cleanup = func() {
		b.removeClient(c.id)
	}

	return c.events, cleanup
}

// ClientCount returns the number of connected clients.
func (b *broker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// broadcastLoop distributes events to all clients.
func (b *broker) broadcastLoop() {
	defer b.wg.Done()

	for {
		select {
		case event := <-b.publish:
			b.broadcast(event)
		case <-b.ctx.Done():
			b.disconnectAllClients()
			return
		}
	}
}

// broadcast sends an event to all clients (with filtering).
func (b *broker) broadcast(event Event) {
	b.mu.RLock()
	clients := make([]*client, 0, len(b.clients))
	for _, c := range b.clients {
		clients = append(clients, c)
	}
	b.mu.RUnlock()

	sent := 0
	dropped := 0
	slowClients := make([]string, 0)

	for _, c := range clients {
		if c.send(event) {
			sent++
		} else {
			dropped++
			slowClients = append(slowClients, c.id)
		}
	}

	// Close slow clients
	for _, clientID := range slowClients {
		b.logger.Warn("Client buffer full, closing slow connection",
			infralogger.String("client_id", clientID),
			infralogger.String("event_type", event.Type),
		)
		b.removeClient(clientID)
	}

	if sent > 0 || dropped > 0 {
		b.logger.Debug("Event broadcast",
			infralogger.String("event_type", event.Type),
			infralogger.Int("sent", sent),
			infralogger.Int("dropped", dropped),
		)
	}
}

// cleanupClient waits for client context to be cancelled and removes it.
func (b *broker) cleanupClient(c *client) {
	defer b.wg.Done()

	<-c.ctx.Done()

	b.removeClient(c.id)
}

// removeClient removes and closes a client.
func (b *broker) removeClient(clientID string) {
	b.mu.Lock()
	c, exists := b.clients[clientID]
	if exists {
		delete(b.clients, clientID)
	}
	b.mu.Unlock()

	if exists && c != nil {
		c.close()
		b.logger.Debug("Client disconnected",
			infralogger.String("client_id", clientID),
			infralogger.Int("total_clients", b.ClientCount()),
		)
	}
}

// disconnectAllClients closes all client connections.
func (b *broker) disconnectAllClients() {
	b.mu.Lock()
	clients := make([]*client, 0, len(b.clients))
	for _, c := range b.clients {
		clients = append(clients, c)
	}
	b.clients = make(map[string]*client)
	b.mu.Unlock()

	for _, c := range clients {
		c.close()
	}

	b.logger.Info("All SSE clients disconnected",
		infralogger.Int("count", len(clients)),
	)
}
