package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type ipEntry struct {
	count     int
	expiresAt time.Time
}

// rateLimitState holds the shared state for the rate limiter.
type rateLimitState struct {
	mu      sync.Mutex
	entries map[string]*ipEntry
}

// cleanup removes expired entries from the map.
func (s *rateLimitState) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for ip, entry := range s.entries {
		if now.After(entry.expiresAt) {
			delete(s.entries, ip)
		}
	}
}

// startCleanup runs periodic cleanup until done is closed.
func (s *rateLimitState) startCleanup(window time.Duration, done <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(window)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.cleanup()
			case <-done:
				return
			}
		}
	}()
}

// RateLimiter limits requests per IP address within a time window.
// The done channel signals the background cleanup goroutine to exit.
func RateLimiter(maxRequests int, window time.Duration, done <-chan struct{}) gin.HandlerFunc {
	state := &rateLimitState{
		entries: make(map[string]*ipEntry),
	}
	state.startCleanup(window, done)

	return func(c *gin.Context) {
		ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		if ip == "" {
			ip = c.Request.RemoteAddr
		}

		state.mu.Lock()
		entry, exists := state.entries[ip]
		now := time.Now()

		if !exists || now.After(entry.expiresAt) {
			state.entries[ip] = &ipEntry{count: 1, expiresAt: now.Add(window)}
			state.mu.Unlock()
			c.Next()
			return
		}

		entry.count++
		if entry.count > maxRequests {
			state.mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}
		state.mu.Unlock()
		c.Next()
	}
}
