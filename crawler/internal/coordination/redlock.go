package coordination

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// clockDriftFactor is the clock drift factor for calculating validity time.
	clockDriftFactor = 0.01

	// minLockTTL is the minimum TTL for Redlock.
	minLockTTL = 100 * time.Millisecond

	// defaultRedlockRetryCount is the default number of retries.
	defaultRedlockRetryCount = 3

	// defaultRedlockRetryDelayMs is the default retry delay in milliseconds.
	defaultRedlockRetryDelayMs = 200

	// quorumDivisor is used to calculate majority (n/2 + 1).
	quorumDivisor = 2
)

var (
	// ErrRedlockNotAcquired is returned when Redlock cannot be acquired.
	ErrRedlockNotAcquired = errors.New("redlock: could not acquire lock on majority of instances")

	// ErrNoRedisInstances is returned when no Redis instances are provided.
	ErrNoRedisInstances = errors.New("redlock: at least one Redis instance is required")
)

// Redlock implements the Redlock distributed locking algorithm.
// It acquires locks across multiple Redis instances for stronger guarantees.
type Redlock struct {
	clients    []*redis.Client
	key        string
	token      string
	ttl        time.Duration
	retryCount int
	retryDelay time.Duration
}

// RedlockConfig holds configuration for Redlock.
type RedlockConfig struct {
	TTL        time.Duration // Lock TTL (default: 30s)
	RetryCount int           // Number of retries (default: 3)
	RetryDelay time.Duration // Delay between retries (default: 200ms)
}

// DefaultRedlockConfig returns a RedlockConfig with sensible defaults.
func DefaultRedlockConfig() RedlockConfig {
	return RedlockConfig{
		TTL:        DefaultLockTTL,
		RetryCount: defaultRedlockRetryCount,
		RetryDelay: defaultRedlockRetryDelayMs * time.Millisecond,
	}
}

// NewRedlock creates a new Redlock instance.
// Requires at least one Redis client, but typically 3 or 5 for proper distributed locking.
func NewRedlock(clients []*redis.Client, key string, cfg RedlockConfig) (*Redlock, error) {
	if len(clients) == 0 {
		return nil, ErrNoRedisInstances
	}

	if cfg.TTL < minLockTTL {
		cfg.TTL = DefaultLockTTL
	}
	if cfg.RetryCount <= 0 {
		cfg.RetryCount = defaultRedlockRetryCount
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = defaultRedlockRetryDelayMs * time.Millisecond
	}

	return &Redlock{
		clients:    clients,
		key:        key,
		token:      uuid.New().String(),
		ttl:        cfg.TTL,
		retryCount: cfg.RetryCount,
		retryDelay: cfg.RetryDelay,
	}, nil
}

// Lock acquires the lock across multiple Redis instances.
func (r *Redlock) Lock(ctx context.Context) error {
	for i := range r.retryCount {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if r.tryLock(ctx) {
			return nil
		}

		// Wait before retrying
		if i < r.retryCount-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(r.retryDelay):
			}
		}
	}

	return ErrRedlockNotAcquired
}

// tryLock attempts to acquire the lock on all instances.
func (r *Redlock) tryLock(ctx context.Context) bool {
	start := time.Now()
	quorum := len(r.clients)/quorumDivisor + 1

	// Try to acquire lock on all instances
	acquired := 0
	args := redis.SetArgs{Mode: "NX", TTL: r.ttl}
	for _, client := range r.clients {
		result, setErr := client.SetArgs(ctx, r.key, r.token, args).Result()
		if setErr != nil {
			continue // Instance failure, try next
		}
		if result == "OK" {
			acquired++
		}
	}

	// Calculate elapsed time and remaining validity
	elapsed := time.Since(start)
	drift := time.Duration(float64(r.ttl) * clockDriftFactor)
	validity := r.ttl - elapsed - drift

	// Check if we have quorum and validity
	if acquired >= quorum && validity > 0 {
		return true
	}

	// Failed to acquire quorum, release all acquired locks
	r.unlock(ctx)
	return false
}

// Unlock releases the lock on all instances.
func (r *Redlock) Unlock(ctx context.Context) error {
	r.unlock(ctx)
	return nil
}

// unlock releases the lock on all instances without returning errors.
func (r *Redlock) unlock(ctx context.Context) {
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`)

	for _, client := range r.clients {
		// Ignore errors during unlock - best effort
		_, _ = script.Run(ctx, client, []string{r.key}, r.token).Int()
	}
}

// Extend extends the lock TTL on all instances.
func (r *Redlock) Extend(ctx context.Context, extension time.Duration) error {
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("pexpire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)

	quorum := len(r.clients)/quorumDivisor + 1
	extended := 0

	for _, client := range r.clients {
		result, err := script.Run(ctx, client, []string{r.key}, r.token, extension.Milliseconds()).Int()
		if err != nil {
			continue
		}
		if result == 1 {
			extended++
		}
	}

	if extended >= quorum {
		return nil
	}

	return fmt.Errorf("redlock: failed to extend lock on majority of instances (%d/%d)", extended, len(r.clients))
}

// IsHeld checks if this instance holds the lock on a majority of instances.
func (r *Redlock) IsHeld(ctx context.Context) (bool, error) {
	quorum := len(r.clients)/quorumDivisor + 1
	held := 0

	for _, client := range r.clients {
		val, err := client.Get(ctx, r.key).Result()
		if err != nil {
			continue
		}
		if val == r.token {
			held++
		}
	}

	return held >= quorum, nil
}

// Key returns the lock key.
func (r *Redlock) Key() string {
	return r.key
}

// Token returns the lock token.
func (r *Redlock) Token() string {
	return r.token
}

// QuorumSize returns the quorum size for this Redlock.
func (r *Redlock) QuorumSize() int {
	return len(r.clients)/quorumDivisor + 1
}

// InstanceCount returns the number of Redis instances.
func (r *Redlock) InstanceCount() int {
	return len(r.clients)
}
