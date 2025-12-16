// Package circuitbreaker provides a circuit breaker implementation for external service calls.
package circuitbreaker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrCircuitOpen is returned when the circuit breaker is open
	ErrCircuitOpen = errors.New("circuit breaker is open")
	// ErrInvalidState is returned when the circuit breaker is in an invalid state
	ErrInvalidState = errors.New("circuit breaker is in invalid state")
)

// State represents the state of the circuit breaker
type State int

const (
	// StateClosed means the circuit is closed and requests are allowed
	StateClosed State = iota
	// StateOpen means the circuit is open and requests are blocked
	StateOpen
	// StateHalfOpen means the circuit is half-open and testing if service recovered
	StateHalfOpen
)

// String returns the string representation of the state
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config configures a circuit breaker
type Config struct {
	// FailureThreshold is the number of consecutive failures before opening the circuit
	FailureThreshold int
	// SuccessThreshold is the number of consecutive successes in half-open state before closing
	SuccessThreshold int
	// Timeout is how long the circuit stays open before transitioning to half-open
	Timeout time.Duration
	// OnStateChange is an optional callback when state changes
	OnStateChange func(from, to State)
}

// DefaultConfig returns a default circuit breaker configuration
func DefaultConfig() Config {
	return Config{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:           60 * time.Second,
	}
}

// Breaker implements a circuit breaker pattern
type Breaker struct {
	mu                sync.RWMutex
	state             State
	failureCount      int
	successCount      int
	lastFailureTime   time.Time
	config            Config
	onStateChange     func(from, to State)
}

// New creates a new circuit breaker with the given configuration
func New(config Config) *Breaker {
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = 5
	}
	if config.SuccessThreshold <= 0 {
		config.SuccessThreshold = 2
	}
	if config.Timeout <= 0 {
		config.Timeout = 60 * time.Second
	}

	return &Breaker{
		state:         StateClosed,
		config:        config,
		onStateChange: config.OnStateChange,
	}
}

// Execute executes a function with circuit breaker protection
func (b *Breaker) Execute(ctx context.Context, fn func() error) error {
	// Check if we can proceed
	if err := b.beforeCall(); err != nil {
		return err
	}

	// Execute the function
	err := fn()

	// Record the result
	b.afterCall(err)

	return err
}

// beforeCall checks if the circuit breaker allows the call
func (b *Breaker) beforeCall() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Check if we should transition from open to half-open
	if b.state == StateOpen {
		if time.Since(b.lastFailureTime) >= b.config.Timeout {
			b.transitionTo(StateHalfOpen)
		} else {
			return fmt.Errorf("%w: circuit will retry after %v", ErrCircuitOpen, b.config.Timeout-time.Since(b.lastFailureTime))
		}
	}

	// Half-open state allows one call to test if service recovered
	if b.state == StateHalfOpen {
		// Allow the call
		return nil
	}

	// Closed state allows all calls
	return nil
}

// afterCall records the result of the call
func (b *Breaker) afterCall(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err != nil {
		b.recordFailure()
	} else {
		b.recordSuccess()
	}
}

// recordFailure records a failure
func (b *Breaker) recordFailure() {
	b.failureCount++
	b.lastFailureTime = time.Now()

	switch b.state {
	case StateClosed:
		if b.failureCount >= b.config.FailureThreshold {
			b.transitionTo(StateOpen)
		}
	case StateHalfOpen:
		// Any failure in half-open state immediately opens the circuit
		b.transitionTo(StateOpen)
	case StateOpen:
		// Already open, just update failure time
	}
}

// recordSuccess records a success
func (b *Breaker) recordSuccess() {
	b.failureCount = 0

	switch b.state {
	case StateClosed:
		// Reset failure count on success
		b.failureCount = 0
	case StateHalfOpen:
		b.successCount++
		if b.successCount >= b.config.SuccessThreshold {
			b.transitionTo(StateClosed)
		}
	case StateOpen:
		// Should not happen, but handle gracefully
	}
}

// transitionTo transitions to a new state
func (b *Breaker) transitionTo(newState State) {
	if b.state == newState {
		return
	}

	oldState := b.state
	b.state = newState

	// Reset counters based on new state
	switch newState {
	case StateClosed:
		b.failureCount = 0
		b.successCount = 0
	case StateOpen:
		b.failureCount = 0
		b.successCount = 0
	case StateHalfOpen:
		b.successCount = 0
	}

	// Call state change callback
	if b.onStateChange != nil {
		b.onStateChange(oldState, newState)
	}
}

// State returns the current state of the circuit breaker
func (b *Breaker) State() State {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// Reset resets the circuit breaker to closed state
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.transitionTo(StateClosed)
}

// Stats returns statistics about the circuit breaker
type Stats struct {
	State         State
	FailureCount  int
	SuccessCount  int
	LastFailureTime time.Time
}

// GetStats returns current statistics
func (b *Breaker) GetStats() Stats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return Stats{
		State:          b.state,
		FailureCount:   b.failureCount,
		SuccessCount:   b.successCount,
		LastFailureTime: b.lastFailureTime,
	}
}

