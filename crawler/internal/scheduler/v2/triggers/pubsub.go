package triggers

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler/v2/schedule"
)

const (
	// defaultReconnectDelay is the default delay between reconnection attempts.
	defaultReconnectDelay = 5 * time.Second

	// defaultHealthCheckInterval is the interval for health checks.
	defaultHealthCheckInterval = 30 * time.Second

	// maxReconnectDelay is the maximum delay between reconnection attempts.
	maxReconnectDelay = 60 * time.Second

	// reconnectBackoffMultiplier is the multiplier for exponential backoff.
	reconnectBackoffMultiplier = 2
)

var (
	// ErrPubSubNotRunning is returned when the Pub/Sub listener is not running.
	ErrPubSubNotRunning = errors.New("pubsub listener not running")

	// ErrAlreadySubscribed is returned when already subscribed to a channel.
	ErrAlreadySubscribed = errors.New("already subscribed to channel")
)

// PubSubMessage represents a message received from Redis Pub/Sub.
type PubSubMessage struct {
	Channel   string         `json:"channel"`
	Event     string         `json:"event,omitempty"`
	Source    string         `json:"source,omitempty"`
	Timestamp time.Time      `json:"timestamp,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}

// PubSubConfig holds configuration for the Pub/Sub listener.
type PubSubConfig struct {
	// ReconnectDelay is the initial delay between reconnection attempts.
	ReconnectDelay time.Duration

	// HealthCheckInterval is the interval for health checks.
	HealthCheckInterval time.Duration

	// MaxReconnectDelay is the maximum delay between reconnection attempts.
	MaxReconnectDelay time.Duration
}

// DefaultPubSubConfig returns a config with sensible defaults.
func DefaultPubSubConfig() PubSubConfig {
	return PubSubConfig{
		ReconnectDelay:      defaultReconnectDelay,
		HealthCheckInterval: defaultHealthCheckInterval,
		MaxReconnectDelay:   maxReconnectDelay,
	}
}

// PubSubListener listens to Redis Pub/Sub channels and triggers jobs.
type PubSubListener struct {
	client       *redis.Client
	matcher      *schedule.EventMatcher
	eventHandler schedule.EventHandler
	config       PubSubConfig

	mu        sync.RWMutex
	pubsub    *redis.PubSub
	channels  map[string]bool
	running   bool
	cancelFn  context.CancelFunc
	healthErr error
}

// NewPubSubListener creates a new Pub/Sub listener.
func NewPubSubListener(
	client *redis.Client,
	matcher *schedule.EventMatcher,
	handler schedule.EventHandler,
	config PubSubConfig,
) *PubSubListener {
	return &PubSubListener{
		client:       client,
		matcher:      matcher,
		eventHandler: handler,
		config:       config,
		channels:     make(map[string]bool),
	}
}

// Start starts the Pub/Sub listener.
func (l *PubSubListener) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.running {
		return nil
	}

	// Create cancellable context
	listenerCtx, cancel := context.WithCancel(ctx)
	l.cancelFn = cancel

	// Get channels to subscribe to
	channels := l.matcher.GetRegisteredChannels()
	if len(channels) == 0 {
		l.running = true
		return nil // No channels to subscribe to yet
	}

	// Create Pub/Sub connection
	l.pubsub = l.client.Subscribe(listenerCtx, channels...)

	// Mark channels as subscribed
	for _, ch := range channels {
		l.channels[ch] = true
	}

	l.running = true

	// Start message processing goroutine
	go l.processMessages(listenerCtx)

	// Start health check goroutine
	go l.healthCheck(listenerCtx)

	return nil
}

// Stop stops the Pub/Sub listener.
func (l *PubSubListener) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.running {
		return nil
	}

	// Cancel context
	if l.cancelFn != nil {
		l.cancelFn()
	}

	// Close Pub/Sub connection
	if l.pubsub != nil {
		if closeErr := l.pubsub.Close(); closeErr != nil {
			return closeErr
		}
	}

	l.running = false
	l.channels = make(map[string]bool)
	return nil
}

// Subscribe adds a channel to the subscription list.
func (l *PubSubListener) Subscribe(ctx context.Context, channel string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.channels[channel] {
		return ErrAlreadySubscribed
	}

	if l.pubsub != nil && l.running {
		if err := l.pubsub.Subscribe(ctx, channel); err != nil {
			return err
		}
	}

	l.channels[channel] = true
	return nil
}

// Unsubscribe removes a channel from the subscription list.
func (l *PubSubListener) Unsubscribe(ctx context.Context, channel string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.channels[channel] {
		return nil // Not subscribed
	}

	if l.pubsub != nil && l.running {
		if err := l.pubsub.Unsubscribe(ctx, channel); err != nil {
			return err
		}
	}

	delete(l.channels, channel)
	return nil
}

// IsRunning returns true if the listener is running.
func (l *PubSubListener) IsRunning() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.running
}

// GetSubscribedChannels returns the list of subscribed channels.
func (l *PubSubListener) GetSubscribedChannels() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	channels := make([]string, 0, len(l.channels))
	for ch := range l.channels {
		channels = append(channels, ch)
	}
	return channels
}

// HealthError returns the last health check error, if any.
func (l *PubSubListener) HealthError() error {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.healthErr
}

// processMessages processes incoming Pub/Sub messages.
func (l *PubSubListener) processMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			l.mu.RLock()
			pubsub := l.pubsub
			l.mu.RUnlock()

			if pubsub == nil {
				return
			}

			// Receive message with timeout
			msg, err := pubsub.ReceiveMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				// Attempt reconnection
				l.handleDisconnect(ctx)
				continue
			}

			// Process the message
			l.handleMessage(ctx, msg)
		}
	}
}

// handleMessage processes a single Pub/Sub message.
func (l *PubSubListener) handleMessage(ctx context.Context, msg *redis.Message) {
	// Parse message payload
	var payload PubSubMessage
	if unmarshalErr := json.Unmarshal([]byte(msg.Payload), &payload); unmarshalErr != nil {
		// Try treating the payload as raw data
		payload = PubSubMessage{
			Channel: msg.Channel,
			Data:    map[string]any{"raw": msg.Payload},
		}
	}
	payload.Channel = msg.Channel

	// Find matching jobs
	jobIDs := l.matcher.MatchChannel(msg.Channel)
	if len(jobIDs) == 0 {
		return
	}

	// Trigger matched jobs
	for _, jobID := range jobIDs {
		event := schedule.Event{
			Type:    schedule.EventTypePubSub,
			Source:  payload.Source,
			Pattern: msg.Channel,
			Payload: payload.Data,
		}

		// Trigger asynchronously to not block message processing
		go func(jid string, evt schedule.Event) {
			_ = l.eventHandler(ctx, jid, evt)
		}(jobID, event)
	}
}

// handleDisconnect handles disconnection and attempts reconnection.
func (l *PubSubListener) handleDisconnect(ctx context.Context) {
	delay := l.config.ReconnectDelay

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
			if l.reconnect(ctx) {
				return
			}

			// Exponential backoff
			delay *= reconnectBackoffMultiplier
			if delay > l.config.MaxReconnectDelay {
				delay = l.config.MaxReconnectDelay
			}
		}
	}
}

// reconnect attempts to reconnect to Redis Pub/Sub.
func (l *PubSubListener) reconnect(ctx context.Context) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Close existing connection
	if l.pubsub != nil {
		_ = l.pubsub.Close()
	}

	// Get channels to resubscribe
	channels := make([]string, 0, len(l.channels))
	for ch := range l.channels {
		channels = append(channels, ch)
	}

	if len(channels) == 0 {
		return true
	}

	// Create new subscription
	l.pubsub = l.client.Subscribe(ctx, channels...)

	// Verify connection
	_, err := l.pubsub.Receive(ctx)
	if err != nil {
		l.healthErr = err
		return false
	}

	l.healthErr = nil
	return true
}

// healthCheck periodically checks the health of the Pub/Sub connection.
func (l *PubSubListener) healthCheck(ctx context.Context) {
	ticker := time.NewTicker(l.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.mu.Lock()
			if l.pubsub != nil {
				// Ping to check connection
				if err := l.client.Ping(ctx).Err(); err != nil {
					l.healthErr = err
				} else {
					l.healthErr = nil
				}
			}
			l.mu.Unlock()
		}
	}
}
