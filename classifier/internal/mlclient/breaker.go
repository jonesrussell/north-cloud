package mlclient

import (
	"sync"
	"time"
)

// breakerState represents the circuit breaker state.
type breakerState int

const (
	breakerClosed   breakerState = iota // Normal operation — requests allowed.
	breakerOpen                         // Too many failures — requests blocked.
	breakerHalfOpen                     // Cooldown expired — one probe request allowed.
)

// circuitBreaker tracks consecutive failures and prevents calls when the circuit is open.
type circuitBreaker struct {
	mu          sync.Mutex
	state       breakerState
	failures    int
	maxFailures int
	cooldown    time.Duration
	openedAt    time.Time
}

// newBreaker creates a circuit breaker that opens after maxFailures consecutive failures
// and stays open for the given cooldown duration.
func newBreaker(maxFailures int, cooldown time.Duration) *circuitBreaker {
	return &circuitBreaker{
		state:       breakerClosed,
		maxFailures: maxFailures,
		cooldown:    cooldown,
	}
}

// allow reports whether a request is permitted.
func (b *circuitBreaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case breakerClosed:
		return true
	case breakerOpen:
		if time.Since(b.openedAt) >= b.cooldown {
			b.state = breakerHalfOpen
			return true
		}
		return false
	case breakerHalfOpen:
		return true
	default:
		return false
	}
}

// recordSuccess resets the breaker to the closed state.
func (b *circuitBreaker) recordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state = breakerClosed
	b.failures = 0
}

// recordFailure increments the failure count and opens the breaker when the threshold is reached.
func (b *circuitBreaker) recordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failures++
	if b.failures >= b.maxFailures {
		b.state = breakerOpen
		b.openedAt = time.Now()
	}
}
