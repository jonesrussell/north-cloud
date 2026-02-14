package adaptive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrStateNotFound is returned when no hash state exists for a source.
var ErrStateNotFound = errors.New("hash state not found")

// Adaptive scheduling constants.
const (
	// MaxAdaptiveInterval is the maximum interval between crawls
	// regardless of backoff.
	MaxAdaptiveInterval = 24 * time.Hour
	// keyPrefix is the Redis key prefix for adaptive scheduling state.
	keyPrefix = "crawler:adaptive:"
	// exponentialBase is the base for exponential backoff calculation.
	exponentialBase = 2.0
)

// HashState holds the adaptive scheduling state for a source.
type HashState struct {
	LastHash        string        `json:"last_hash"`
	LastChangeAt    time.Time     `json:"last_change_at"`
	UnchangedCount  int           `json:"unchanged_count"`
	CurrentInterval time.Duration `json:"current_interval"`
}

// HashTracker stores and compares content hashes in Redis
// for adaptive scheduling.
type HashTracker struct {
	client *redis.Client
}

// NewHashTracker creates a new hash tracker.
func NewHashTracker(client *redis.Client) *HashTracker {
	return &HashTracker{client: client}
}

// ComputeHash returns the hex-encoded SHA-256 of content.
func ComputeHash(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

// CalculateAdaptiveInterval computes the next crawl interval based
// on unchanged count.
// Formula: baseline * 2^(unchangedCount), capped at maxInterval.
func CalculateAdaptiveInterval(
	baseline, maxInterval time.Duration,
	unchangedCount int,
) time.Duration {
	if unchangedCount <= 0 {
		return baseline
	}

	multiplier := math.Pow(exponentialBase, float64(unchangedCount))
	interval := time.Duration(float64(baseline) * multiplier)

	if interval > maxInterval {
		return maxInterval
	}

	return interval
}

// CompareAndUpdate compares a new hash against the stored hash for a
// source. Returns the updated state and whether the content changed.
func (ht *HashTracker) CompareAndUpdate(
	ctx context.Context,
	sourceID, newHash string,
	baseline time.Duration,
) (*HashState, bool, error) {
	state, err := ht.loadState(ctx, sourceID)
	if err != nil {
		return nil, false, err
	}

	changed := state.LastHash != newHash

	if changed {
		applyChanged(state, newHash, baseline)
	} else {
		applyUnchanged(state, baseline)
	}

	saveErr := ht.saveState(ctx, sourceID, state)
	if saveErr != nil {
		return nil, false, saveErr
	}

	return state, changed, nil
}

// GetState retrieves the current hash state for a source.
func (ht *HashTracker) GetState(
	ctx context.Context,
	sourceID string,
) (*HashState, error) {
	key := keyPrefix + sourceID

	data, err := ht.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrStateNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get hash state: %w", err)
	}

	var state HashState
	if unmarshalErr := json.Unmarshal(data, &state); unmarshalErr != nil {
		return nil, fmt.Errorf(
			"failed to unmarshal hash state: %w", unmarshalErr,
		)
	}

	return &state, nil
}

// loadState retrieves an existing hash state from Redis, or returns
// a zero-value state if none exists.
func (ht *HashTracker) loadState(
	ctx context.Context,
	sourceID string,
) (*HashState, error) {
	key := keyPrefix + sourceID

	var state HashState

	data, err := ht.client.Get(ctx, key).Bytes()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("failed to get hash state: %w", err)
	}

	if err == nil {
		if unmarshalErr := json.Unmarshal(data, &state); unmarshalErr != nil {
			return nil, fmt.Errorf(
				"failed to unmarshal hash state: %w", unmarshalErr,
			)
		}
	}

	return &state, nil
}

// saveState persists the hash state to Redis.
func (ht *HashTracker) saveState(
	ctx context.Context,
	sourceID string,
	state *HashState,
) error {
	key := keyPrefix + sourceID

	stateBytes, marshalErr := json.Marshal(state)
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal hash state: %w", marshalErr)
	}

	if setErr := ht.client.Set(ctx, key, stateBytes, 0).Err(); setErr != nil {
		return fmt.Errorf("failed to set hash state: %w", setErr)
	}

	return nil
}

// applyChanged updates state when content has changed.
func applyChanged(state *HashState, newHash string, baseline time.Duration) {
	state.LastHash = newHash
	state.LastChangeAt = time.Now()
	state.UnchangedCount = 0
	state.CurrentInterval = baseline
}

// applyUnchanged updates state when content is unchanged.
func applyUnchanged(state *HashState, baseline time.Duration) {
	state.UnchangedCount++
	state.CurrentInterval = CalculateAdaptiveInterval(
		baseline, MaxAdaptiveInterval, state.UnchangedCount,
	)
}
