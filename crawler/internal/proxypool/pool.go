// Package proxypool provides a domain-sticky, round-robin proxy rotation pool.
// It assigns proxies to domains and keeps the assignment sticky for a configurable TTL,
// then rotates via round-robin when the TTL expires or for new domains.
package proxypool

import (
	"errors"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// ErrNoProxies is returned when the pool has no configured proxies.
var ErrNoProxies = errors.New("proxypool: no proxies configured")

// Default durations for sticky TTL and health backoff.
const (
	DefaultStickyTTL     = 10 * time.Minute
	DefaultHealthBackoff = 5 * time.Minute
)

// Option configures a Pool.
type Option func(*Pool)

// WithStickyTTL sets the duration a domain-proxy assignment remains sticky.
func WithStickyTTL(d time.Duration) Option {
	return func(p *Pool) {
		p.stickyTTL = d
	}
}

// WithHealthBackoff sets the duration a proxy is considered unhealthy after being marked.
func WithHealthBackoff(d time.Duration) Option {
	return func(p *Pool) {
		p.healthBackoff = d
	}
}

// domainEntry tracks a domain's assigned proxy and when it was assigned.
type domainEntry struct {
	proxy      *url.URL
	assignedAt time.Time
}

// Pool manages a set of proxy URLs with domain-sticky, round-robin rotation.
type Pool struct {
	proxies       []*url.URL
	stickyTTL     time.Duration
	healthBackoff time.Duration

	mu      sync.RWMutex
	domains map[string]domainEntry
	health  map[string]time.Time // proxy URL string -> unhealthy-until timestamp

	robinIdx atomic.Uint64
}

// New creates a Pool from the given proxy URL strings. Invalid URLs are skipped.
func New(urls []string, opts ...Option) *Pool {
	proxies := make([]*url.URL, 0, len(urls))

	for _, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}

		proxies = append(proxies, u)
	}

	p := &Pool{
		proxies:       proxies,
		stickyTTL:     DefaultStickyTTL,
		healthBackoff: DefaultHealthBackoff,
		domains:       make(map[string]domainEntry),
		health:        make(map[string]time.Time),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// ProxyFor returns the proxy assigned to the given domain. If the domain has a
// sticky assignment within the TTL, the same proxy is returned. Otherwise, the
// next healthy proxy is assigned via round-robin.
func (p *Pool) ProxyFor(domain string) (*url.URL, error) {
	if len(p.proxies) == 0 {
		return nil, ErrNoProxies
	}

	now := time.Now()

	// Check for existing sticky assignment.
	if proxy, ok := p.lookupSticky(domain, now); ok {
		return proxy, nil
	}

	// Assign next healthy proxy via round-robin.
	proxy := p.nextHealthy(now)

	p.mu.Lock()
	p.domains[domain] = domainEntry{proxy: proxy, assignedAt: now}
	p.mu.Unlock()

	return proxy, nil
}

// lookupSticky returns the currently assigned proxy for a domain if it is still
// within the sticky TTL.
func (p *Pool) lookupSticky(domain string, now time.Time) (*url.URL, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	entry, ok := p.domains[domain]
	if !ok {
		return nil, false
	}

	if now.Sub(entry.assignedAt) > p.stickyTTL {
		return nil, false
	}

	return entry.proxy, true
}

// MarkUnhealthy marks the given proxy as unhealthy for the configured backoff duration.
func (p *Pool) MarkUnhealthy(proxy *url.URL) {
	until := time.Now().Add(p.healthBackoff)

	p.mu.Lock()
	p.health[proxy.String()] = until
	p.mu.Unlock()
}

// URLs returns the string representation of all configured proxy URLs for
// logging and diagnostics.
func (p *Pool) URLs() []string {
	result := make([]string, len(p.proxies))
	for i, u := range p.proxies {
		result[i] = u.String()
	}

	return result
}

// nextHealthy returns the next healthy proxy via round-robin. If all proxies
// are unhealthy, it falls back to the next proxy regardless of health (best-effort).
func (p *Pool) nextHealthy(now time.Time) *url.URL {
	count := uint64(len(p.proxies))

	// Try each proxy once, starting from the current robin index.
	startIdx := p.robinIdx.Add(1) - 1

	p.mu.RLock()
	defer p.mu.RUnlock()

	for i := range count {
		candidate := p.proxies[(startIdx+i)%count]

		unhealthyUntil, marked := p.health[candidate.String()]
		if !marked || now.After(unhealthyUntil) {
			return candidate
		}
	}

	// All unhealthy: best-effort fallback to the robin-selected proxy.
	return p.proxies[startIdx%count]
}
