package sse

import "time"

// Default configuration values.
const (
	DefaultEventBufferSize   = 1000
	DefaultClientBufferSize  = 100
	DefaultHeartbeatInterval = 15 * time.Second
	DefaultShutdownTimeout   = 5 * time.Second
	DefaultMaxClients        = 1000
)

// Config holds broker configuration.
type Config struct {
	// EventBufferSize is the size of the main event channel.
	EventBufferSize int
	// ClientBufferSize is the default buffer size per client.
	ClientBufferSize int
	// HeartbeatInterval is how often to send heartbeat comments.
	HeartbeatInterval time.Duration
	// ShutdownTimeout is the maximum time to wait for graceful shutdown.
	ShutdownTimeout time.Duration
	// MaxClients is the maximum number of concurrent clients (0 = unlimited).
	MaxClients int
	// Enabled controls whether SSE is enabled.
	Enabled bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		EventBufferSize:   DefaultEventBufferSize,
		ClientBufferSize:  DefaultClientBufferSize,
		HeartbeatInterval: DefaultHeartbeatInterval,
		ShutdownTimeout:   DefaultShutdownTimeout,
		MaxClients:        DefaultMaxClients,
		Enabled:           true,
	}
}

// BrokerOption configures a broker.
type BrokerOption func(*broker)

// WithEventBufferSize sets the event buffer size.
func WithEventBufferSize(size int) BrokerOption {
	return func(b *broker) {
		if size > 0 {
			b.eventBufferSize = size
		}
	}
}

// WithClientBufferSize sets the default client buffer size.
func WithClientBufferSize(size int) BrokerOption {
	return func(b *broker) {
		if size > 0 {
			b.clientBufferSize = size
		}
	}
}

// WithHeartbeatInterval sets the heartbeat interval.
func WithHeartbeatInterval(interval time.Duration) BrokerOption {
	return func(b *broker) {
		if interval > 0 {
			b.heartbeatInterval = interval
		}
	}
}

// WithShutdownTimeout sets the shutdown timeout.
func WithShutdownTimeout(timeout time.Duration) BrokerOption {
	return func(b *broker) {
		if timeout > 0 {
			b.shutdownTimeout = timeout
		}
	}
}

// WithMaxClients sets the maximum number of concurrent clients.
func WithMaxClients(maxClients int) BrokerOption {
	return func(b *broker) {
		b.maxClients = maxClients
	}
}

// WithConfig applies a full Config to the broker.
func WithConfig(cfg Config) BrokerOption {
	return func(b *broker) {
		if cfg.EventBufferSize > 0 {
			b.eventBufferSize = cfg.EventBufferSize
		}
		if cfg.ClientBufferSize > 0 {
			b.clientBufferSize = cfg.ClientBufferSize
		}
		if cfg.HeartbeatInterval > 0 {
			b.heartbeatInterval = cfg.HeartbeatInterval
		}
		if cfg.ShutdownTimeout > 0 {
			b.shutdownTimeout = cfg.ShutdownTimeout
		}
		b.maxClients = cfg.MaxClients
	}
}

// ClientOption configures a client subscription.
type ClientOption func(*ClientOptions)

// WithFilter sets an event filter for the client.
func WithFilter(filter EventFilter) ClientOption {
	return func(opts *ClientOptions) {
		opts.Filter = filter
	}
}

// WithBufferSize sets the client's event buffer size.
func WithBufferSize(size int) ClientOption {
	return func(opts *ClientOptions) {
		if size > 0 {
			opts.BufferSize = size
		}
	}
}

// WithJobFilter creates a filter that only passes job-related events.
func WithJobFilter() ClientOption {
	return WithFilter(func(event Event) bool {
		switch event.Type {
		case EventTypeJobStatus, EventTypeJobProgress, EventTypeJobCompleted:
			return true
		default:
			return false
		}
	})
}

// WithHealthFilter creates a filter that only passes health events.
func WithHealthFilter() ClientOption {
	return WithFilter(func(event Event) bool {
		return event.Type == EventTypeHealthStatus
	})
}

// WithMetricsFilter creates a filter that only passes metrics events.
func WithMetricsFilter() ClientOption {
	return WithFilter(func(event Event) bool {
		switch event.Type {
		case EventTypeMetricsUpdate, EventTypePipelineStage:
			return true
		default:
			return false
		}
	})
}
