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

// RateLimiter limits requests per IP address within a time window.
func RateLimiter(maxRequests int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	entries := make(map[string]*ipEntry)

	// Background cleanup every window duration
	go func() {
		ticker := time.NewTicker(window)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			now := time.Now()
			for ip, entry := range entries {
				if now.After(entry.expiresAt) {
					delete(entries, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		if ip == "" {
			ip = c.Request.RemoteAddr
		}

		mu.Lock()
		entry, exists := entries[ip]
		now := time.Now()

		if !exists || now.After(entry.expiresAt) {
			entries[ip] = &ipEntry{count: 1, expiresAt: now.Add(window)}
			mu.Unlock()
			c.Next()
			return
		}

		entry.count++
		if entry.count > maxRequests {
			mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}
		mu.Unlock()
		c.Next()
	}
}
