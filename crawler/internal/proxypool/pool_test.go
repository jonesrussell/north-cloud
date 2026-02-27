package proxypool_test

import (
	"errors"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/proxypool"
)

const (
	testProxy1 = "http://proxy1.example.com:8080"
	testProxy2 = "http://proxy2.example.com:8080"
	testProxy3 = "http://proxy3.example.com:8080"

	testDomainA = "domainA.com"
	testDomainB = "domainB.com"
	testDomainC = "domainC.com"

	concurrentGoroutines = 50
	concurrentIterations = 100
)

func testProxyURLs() []string {
	return []string{testProxy1, testProxy2, testProxy3}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("failed to parse URL %q: %v", raw, err)
	}

	return u
}

func requireNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func requireError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func requireEqual(t *testing.T, expected, actual string) {
	t.Helper()

	if expected != actual {
		t.Fatalf("expected %q, got %q", expected, actual)
	}
}

func requireNotEqual(t *testing.T, a, b string) {
	t.Helper()

	if a == b {
		t.Fatalf("expected values to differ, both are %q", a)
	}
}

func TestPool_RoundRobinAssignment(t *testing.T) {
	t.Parallel()

	pool := proxypool.New(testProxyURLs())

	// Three different domains should get assigned in round-robin order.
	proxy1, err := pool.ProxyFor(testDomainA)
	requireNoError(t, err)

	proxy2, err := pool.ProxyFor(testDomainB)
	requireNoError(t, err)

	proxy3, err := pool.ProxyFor(testDomainC)
	requireNoError(t, err)

	// All three should be distinct proxies.
	requireNotEqual(t, proxy1.String(), proxy2.String())
	requireNotEqual(t, proxy2.String(), proxy3.String())
	requireNotEqual(t, proxy1.String(), proxy3.String())
}

func TestPool_DomainSticky(t *testing.T) {
	t.Parallel()

	pool := proxypool.New(testProxyURLs())

	first, err := pool.ProxyFor(testDomainA)
	requireNoError(t, err)

	// Same domain should return the same proxy within TTL.
	second, err := pool.ProxyFor(testDomainA)
	requireNoError(t, err)

	requireEqual(t, first.String(), second.String())
}

func TestPool_StickyTTLExpiry(t *testing.T) {
	t.Parallel()

	shortTTL := 1 * time.Millisecond
	pool := proxypool.New(testProxyURLs(), proxypool.WithStickyTTL(shortTTL))

	first, err := pool.ProxyFor(testDomainA)
	requireNoError(t, err)

	// Wait for the TTL to expire.
	time.Sleep(2 * time.Millisecond)

	second, err := pool.ProxyFor(testDomainA)
	requireNoError(t, err)

	// After TTL expiry, round-robin advances so proxy may change.
	// We verify it doesn't panic and returns a valid proxy, not
	// necessarily a different one (depends on robin index).
	if second == nil {
		t.Fatal("expected a valid proxy after TTL expiry, got nil")
	}

	_ = first // first was used above; suppress unused warning clarity
}

func TestPool_MarkUnhealthy(t *testing.T) {
	t.Parallel()

	pool := proxypool.New(testProxyURLs())

	// Assign proxy to domainA.
	first, err := pool.ProxyFor(testDomainA)
	requireNoError(t, err)

	// Mark it unhealthy.
	pool.MarkUnhealthy(first)

	// New domain should skip the unhealthy proxy.
	second, err := pool.ProxyFor(testDomainB)
	requireNoError(t, err)

	requireNotEqual(t, first.String(), second.String())
}

func TestPool_AllUnhealthy(t *testing.T) {
	t.Parallel()

	pool := proxypool.New(testProxyURLs())

	// Mark all proxies unhealthy.
	for _, raw := range testProxyURLs() {
		pool.MarkUnhealthy(mustParseURL(t, raw))
	}

	// Should still return a proxy (best-effort fallback).
	proxy, err := pool.ProxyFor(testDomainA)
	requireNoError(t, err)

	if proxy == nil {
		t.Fatal("expected best-effort proxy when all are unhealthy, got nil")
	}
}

func TestPool_EmptyURLs(t *testing.T) {
	t.Parallel()

	pool := proxypool.New(nil)

	_, err := pool.ProxyFor(testDomainA)
	requireError(t, err)

	if !errors.Is(err, proxypool.ErrNoProxies) {
		t.Fatalf("expected ErrNoProxies, got %v", err)
	}
}

func TestPool_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	pool := proxypool.New(testProxyURLs())
	domains := []string{"a.com", "b.com", "c.com", "d.com", "e.com"}

	var wg sync.WaitGroup
	wg.Add(concurrentGoroutines)

	for range concurrentGoroutines {
		go func() {
			defer wg.Done()

			for i := range concurrentIterations {
				domain := domains[i%len(domains)]

				proxy, err := pool.ProxyFor(domain)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}

				if proxy == nil {
					t.Error("unexpected nil proxy")
					return
				}

				// Periodically mark unhealthy to stress concurrency.
				if i%len(domains) == 0 {
					pool.MarkUnhealthy(proxy)
				}
			}
		}()
	}

	wg.Wait()
}

func TestPool_URLs(t *testing.T) {
	t.Parallel()

	urls := testProxyURLs()
	pool := proxypool.New(urls)
	got := pool.URLs()

	if len(got) != len(urls) {
		t.Fatalf("expected %d URLs, got %d", len(urls), len(got))
	}

	for i, u := range got {
		requireEqual(t, urls[i], u)
	}
}
