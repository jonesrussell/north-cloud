package proxypool_test

import (
	"net/http"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/proxypool"
)

const (
	transportTestProxy1 = "http://proxy1.example.com:9090"
	transportTestProxy2 = "http://proxy2.example.com:9090"

	baseMaxIdleConns = 42
)

func transportProxyURLs() []string {
	return []string{transportTestProxy1, transportTestProxy2}
}

func TestProxyFunc_RoutesToCorrectProxy(t *testing.T) {
	t.Parallel()

	pool := mustNewPool(t, transportProxyURLs())
	proxyFn := pool.ProxyFunc()

	req, err := http.NewRequest(http.MethodGet, "http://example.com/page", http.NoBody)
	requireNoError(t, err)

	first, err := proxyFn(req)
	requireNoError(t, err)

	if first == nil {
		t.Fatal("expected non-nil proxy URL")
	}

	// Same domain should return the same proxy (sticky).
	second, err := proxyFn(req)
	requireNoError(t, err)

	requireEqual(t, first.String(), second.String())
}

func TestProxyFunc_DifferentDomainsGetDifferentProxies(t *testing.T) {
	t.Parallel()

	pool := mustNewPool(t, transportProxyURLs())
	proxyFn := pool.ProxyFunc()

	req1, err := http.NewRequest(http.MethodGet, "http://alpha.com/page", http.NoBody)
	requireNoError(t, err)

	req2, err := http.NewRequest(http.MethodGet, "http://beta.com/page", http.NoBody)
	requireNoError(t, err)

	proxy1, err := proxyFn(req1)
	requireNoError(t, err)

	proxy2, err := proxyFn(req2)
	requireNoError(t, err)

	requireNotEqual(t, proxy1.String(), proxy2.String())
}

func TestNewTransport_SetsProxyFunc(t *testing.T) {
	t.Parallel()

	pool := mustNewPool(t, transportProxyURLs())
	transport := proxypool.NewTransport(pool, nil)

	if transport.Proxy == nil {
		t.Fatal("expected transport.Proxy to be set")
	}
}

func TestNewTransport_PreservesBaseSettings(t *testing.T) {
	t.Parallel()

	base := &http.Transport{
		MaxIdleConns: baseMaxIdleConns,
	}

	pool := mustNewPool(t, transportProxyURLs())
	transport := proxypool.NewTransport(pool, base)

	if transport.MaxIdleConns != baseMaxIdleConns {
		t.Fatalf("expected MaxIdleConns=%d, got %d", baseMaxIdleConns, transport.MaxIdleConns)
	}

	if transport.Proxy == nil {
		t.Fatal("expected transport.Proxy to be set after clone")
	}
}

func TestNewTransport_NilBaseUsesDefaults(t *testing.T) {
	t.Parallel()

	pool := mustNewPool(t, transportProxyURLs())
	transport := proxypool.NewTransport(pool, nil)

	// Should inherit Go's default transport settings (non-zero MaxIdleConns).
	defaultTransport := http.DefaultTransport.(*http.Transport)
	if transport.MaxIdleConns != defaultTransport.MaxIdleConns {
		t.Fatalf("expected default MaxIdleConns=%d, got %d",
			defaultTransport.MaxIdleConns, transport.MaxIdleConns)
	}
}
