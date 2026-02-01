package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Proxy is the main HTTP proxy handler.
type Proxy struct {
	cfg   *Config
	cache *Cache
	mu    sync.RWMutex
	mode  Mode

	// HTTP client for live requests
	client *http.Client
}

// NewProxy creates a new proxy instance.
func NewProxy(cfg *Config) *Proxy {
	return &Proxy{
		cfg:   cfg,
		cache: NewCache(cfg.FixturesDir, cfg.CacheDir),
		mode:  cfg.Mode,
		client: &http.Client{
			Timeout: cfg.LiveTimeout,
		},
	}
}

// Mode returns the current operating mode.
func (p *Proxy) Mode() Mode {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mode
}

// SetMode changes the operating mode.
func (p *Proxy) SetMode(mode Mode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mode = mode
}

// Cache returns the proxy's cache instance.
func (p *Proxy) Cache() *Cache {
	return p.cache
}

// ServeHTTP handles proxy requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle CONNECT method separately (for HTTPS)
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
		return
	}

	p.handleHTTP(w, r)
}

func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	mode := p.Mode()
	domain := NormalizeDomain(r.URL.Host)
	cacheKey := GenerateCacheKey(r)

	// Try cache lookup for replay and record modes
	if mode == ModeReplay || mode == ModeRecord {
		entry, source, err := p.cache.Lookup(domain, cacheKey)
		if err == nil && entry != nil {
			p.serveCachedResponse(w, entry, source)
			return
		}
	}

	// Cache miss handling
	switch mode {
	case ModeReplay:
		p.serveCacheMissError(w, r, cacheKey)
	case ModeRecord:
		p.fetchAndCache(w, r, domain, cacheKey)
	case ModeLive:
		p.proxyLive(w, r)
	}
}

func (p *Proxy) handleConnect(w http.ResponseWriter, _ *http.Request) {
	// HTTPS CONNECT handling will be implemented later
	http.Error(w, "CONNECT not yet implemented", http.StatusNotImplemented)
}

func (p *Proxy) serveCachedResponse(w http.ResponseWriter, entry *CacheEntry, source CacheSource) {
	// Copy response headers
	for key, value := range entry.Metadata.Response.Headers {
		w.Header().Set(key, value)
	}

	w.Header().Set("X-Proxy-Mode", string(p.Mode()))
	w.Header().Set("X-Proxy-Source", string(source))

	w.WriteHeader(entry.Metadata.Response.Status)
	_, _ = w.Write(entry.Body)
}

func (p *Proxy) serveCacheMissError(w http.ResponseWriter, r *http.Request, cacheKey string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Proxy-Mode", "replay")
	w.Header().Set("X-Proxy-Cache-Miss", "true")
	w.Header().Set("X-Proxy-Source", "none")
	w.WriteHeader(http.StatusBadGateway)

	errorResponse := map[string]string{
		"error":     "cache_miss",
		"mode":      "replay",
		"url":       r.URL.String(),
		"cache_key": cacheKey,
		"message":   "No fixture or recording found. Run 'task proxy:record' to capture this URL.",
	}

	if encodeErr := json.NewEncoder(w).Encode(errorResponse); encodeErr != nil {
		// Response headers already written, can't change status
		fmt.Printf("warning: failed to encode error response: %v\n", encodeErr)
	}
}

func (p *Proxy) fetchAndCache(w http.ResponseWriter, r *http.Request, domain, cacheKey string) {
	// Create outbound request
	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, r.URL.String(), r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create request: %v", err), http.StatusBadGateway)
		return
	}

	// Copy headers
	copyHeaders(outReq.Header, r.Header)

	// Make request
	resp, err := p.client.Do(outReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("live fetch failed: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read response: %v", err), http.StatusBadGateway)
		return
	}

	// Store in cache
	entry := &CacheEntry{
		Domain:   domain,
		CacheKey: cacheKey,
		BaseDir:  p.cfg.CacheDir,
		Metadata: &CacheEntryMetadata{
			Request: CachedRequest{
				Method:  r.Method,
				URL:     r.URL.String(),
				Headers: flattenHeaders(r.Header),
			},
			Response: CachedResponse{
				Status:  resp.StatusCode,
				Headers: flattenHeaders(resp.Header),
			},
			RecordedAt: time.Now().UTC(),
			CacheKey:   cacheKey,
		},
		Body: body,
	}

	if storeErr := p.cache.Store(entry); storeErr != nil {
		// Log but don't fail the request
		fmt.Printf("warning: failed to cache response: %v\n", storeErr)
	}

	// Send response to client
	p.serveCachedResponse(w, entry, SourceCache)
}

func (p *Proxy) proxyLive(w http.ResponseWriter, r *http.Request) {
	// Create outbound request
	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, r.URL.String(), r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create request: %v", err), http.StatusBadGateway)
		return
	}

	// Copy headers
	copyHeaders(outReq.Header, r.Header)

	// Make request
	resp, err := p.client.Do(outReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("live request failed: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.Header().Set("X-Proxy-Mode", "live")
	w.WriteHeader(resp.StatusCode)

	// Copy body
	_, _ = io.Copy(w, resp.Body)
}

// copyHeaders copies headers from src to dst.
func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// flattenHeaders converts http.Header to map[string]string (first value only).
func flattenHeaders(h http.Header) map[string]string {
	result := make(map[string]string, len(h))
	for key, values := range h {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return result
}

// ExtractDomain extracts and normalizes the domain from a URL.
func ExtractDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return NormalizeDomain(parsed.Host)
}
