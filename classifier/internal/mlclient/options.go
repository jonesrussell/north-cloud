package mlclient

import "time"

const (
	defaultTimeout      = 5 * time.Second
	defaultRetryCount   = 1
	defaultRetryDelay   = 100 * time.Millisecond
	defaultBreakerTrips = 5
	defaultCooldown     = 30 * time.Second
)

// clientOptions holds configuration for a Client.
type clientOptions struct {
	timeout               time.Duration
	retryCount            int
	retryBaseDelay        time.Duration
	breakerTrips          int
	breakerCooldown       time.Duration
	expectedSchemaVersion string
}

func defaultOptions() clientOptions {
	return clientOptions{
		timeout:         defaultTimeout,
		retryCount:      defaultRetryCount,
		retryBaseDelay:  defaultRetryDelay,
		breakerTrips:    defaultBreakerTrips,
		breakerCooldown: defaultCooldown,
	}
}

// Option configures a Client.
type Option func(*clientOptions)

// WithTimeout sets the per-request timeout.
func WithTimeout(d time.Duration) Option {
	return func(o *clientOptions) {
		o.timeout = d
	}
}

// WithRetry sets the number of retry attempts and the base delay between them.
func WithRetry(count int, baseDelay time.Duration) Option {
	return func(o *clientOptions) {
		o.retryCount = count
		o.retryBaseDelay = baseDelay
	}
}

// WithCircuitBreaker configures the circuit breaker trip threshold and cooldown.
func WithCircuitBreaker(trips int, cooldown time.Duration) Option {
	return func(o *clientOptions) {
		o.breakerTrips = trips
		o.breakerCooldown = cooldown
	}
}

// WithExpectedSchemaVersion sets the expected schema version.
// If set, Classify returns ErrSchemaVersion when the response version does not match.
func WithExpectedSchemaVersion(v string) Option {
	return func(o *clientOptions) {
		o.expectedSchemaVersion = v
	}
}
