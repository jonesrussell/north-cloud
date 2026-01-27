// Package coordination provides distributed coordination primitives using Redis.
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
	// DefaultLockTTL is the default lock time-to-live.
	DefaultLockTTL = 30 * time.Second

	// DefaultRetryDelay is the default delay between lock acquisition retries.
	DefaultRetryDelay = 100 * time.Millisecond

	// DefaultMaxRetries is the default maximum number of lock acquisition retries.
	DefaultMaxRetries = 10
)

var (
	// ErrLockNotAcquired is returned when a lock cannot be acquired.
	ErrLockNotAcquired = errors.New("lock not acquired")

	// ErrLockNotHeld is returned when trying to release a lock that is not held.
	ErrLockNotHeld = errors.New("lock not held")
)

// DistributedLock represents a distributed lock using Redis.
type DistributedLock struct {
	client     *redis.Client
	key        string
	token      string
	ttl        time.Duration
	retryDelay time.Duration
	maxRetries int
}

// LockConfig holds configuration for a distributed lock.
type LockConfig struct {
	TTL        time.Duration // Lock TTL (default: 30s)
	RetryDelay time.Duration // Delay between retries (default: 100ms)
	MaxRetries int           // Maximum retries (default: 10)
}

// DefaultLockConfig returns a LockConfig with sensible defaults.
func DefaultLockConfig() LockConfig {
	return LockConfig{
		TTL:        DefaultLockTTL,
		RetryDelay: DefaultRetryDelay,
		MaxRetries: DefaultMaxRetries,
	}
}

// NewDistributedLock creates a new distributed lock.
func NewDistributedLock(client *redis.Client, key string, cfg LockConfig) *DistributedLock {
	if cfg.TTL <= 0 {
		cfg.TTL = DefaultLockTTL
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = DefaultRetryDelay
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = DefaultMaxRetries
	}

	return &DistributedLock{
		client:     client,
		key:        key,
		token:      uuid.New().String(),
		ttl:        cfg.TTL,
		retryDelay: cfg.RetryDelay,
		maxRetries: cfg.MaxRetries,
	}
}

// Lock acquires the lock, blocking until acquired or context is cancelled.
func (l *DistributedLock) Lock(ctx context.Context) error {
	for i := range l.maxRetries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		acquired, err := l.TryLock(ctx)
		if err != nil {
			return err
		}
		if acquired {
			return nil
		}

		// Wait before retrying
		if i < l.maxRetries-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(l.retryDelay):
			}
		}
	}

	return ErrLockNotAcquired
}

// TryLock attempts to acquire the lock without blocking.
// Returns true if the lock was acquired, false otherwise.
func (l *DistributedLock) TryLock(ctx context.Context) (bool, error) {
	ok, err := l.client.SetNX(ctx, l.key, l.token, l.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}
	return ok, nil
}

// Unlock releases the lock if it is held by this instance.
func (l *DistributedLock) Unlock(ctx context.Context) error {
	// Use Lua script to atomically check and delete
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`)

	result, err := script.Run(ctx, l.client, []string{l.key}, l.token).Int()
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}
	if result == 0 {
		return ErrLockNotHeld
	}
	return nil
}

// Extend extends the lock TTL if it is still held.
func (l *DistributedLock) Extend(ctx context.Context, extension time.Duration) error {
	// Use Lua script to atomically check and extend
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("pexpire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)

	result, err := script.Run(ctx, l.client, []string{l.key}, l.token, extension.Milliseconds()).Int()
	if err != nil {
		return fmt.Errorf("failed to extend lock: %w", err)
	}
	if result == 0 {
		return ErrLockNotHeld
	}
	return nil
}

// IsHeld checks if this instance holds the lock.
func (l *DistributedLock) IsHeld(ctx context.Context) (bool, error) {
	val, err := l.client.Get(ctx, l.key).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check lock: %w", err)
	}
	return val == l.token, nil
}

// TTL returns the remaining TTL of the lock.
func (l *DistributedLock) TTL(ctx context.Context) (time.Duration, error) {
	ttl, err := l.client.TTL(ctx, l.key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get lock TTL: %w", err)
	}
	return ttl, nil
}

// Key returns the lock key.
func (l *DistributedLock) Key() string {
	return l.key
}

// Token returns the lock token.
func (l *DistributedLock) Token() string {
	return l.token
}
