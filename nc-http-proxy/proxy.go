package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
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

	// Certificate manager for HTTPS MITM
	certMgr *CertManager
}

// NewProxy creates a new proxy instance.
func NewProxy(cfg *Config) (*Proxy, error) {
	certMgr, err := NewCertManager(cfg.CertsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize certificate manager: %w", err)
	}

	return &Proxy{
		cfg:     cfg,
		cache:   NewCache(cfg.FixturesDir, cfg.CacheDir),
		mode:    cfg.Mode,
		certMgr: certMgr,
		client: &http.Client{
			Timeout: cfg.LiveTimeout,
		},
	}, nil
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

func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract host from CONNECT request (host:port format)
	host := r.Host
	if !strings.Contains(host, ":") {
		host += ":443"
	}

	// Extract just the hostname for certificate generation
	hostname := strings.Split(host, ":")[0]

	// Hijack the connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, fmt.Sprintf("hijack failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Send 200 Connection Established
	_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		clientConn.Close()
		return
	}

	// Get certificate for this host
	cert, err := p.certMgr.GetCertificate(hostname)
	if err != nil {
		fmt.Printf("failed to get certificate for %s: %v\n", hostname, err)
		clientConn.Close()
		return
	}

	// Upgrade to TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		MinVersion:   tls.VersionTLS12,
	}

	tlsConn := tls.Server(clientConn, tlsConfig)
	if handshakeErr := tlsConn.HandshakeContext(ctx); handshakeErr != nil {
		fmt.Printf("TLS handshake failed for %s: %v\n", hostname, handshakeErr)
		tlsConn.Close()
		return
	}

	// Handle HTTP requests over the TLS connection
	p.serveHTTPSConnection(tlsConn, hostname)
}

// serveHTTPSConnection handles HTTP requests over an established TLS connection.
func (p *Proxy) serveHTTPSConnection(conn net.Conn, hostname string) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	for {
		// Read HTTP request
		req, readErr := http.ReadRequest(reader)
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				fmt.Printf("failed to read request: %v\n", readErr)
			}
			return
		}

		// Reconstruct the full URL (request only has path)
		req.URL.Scheme = "https"
		req.URL.Host = hostname
		req.RequestURI = "" // Must be empty for client requests

		// Create a response writer that writes to the connection
		rw := &connResponseWriter{
			conn:    conn,
			headers: make(http.Header),
		}

		// Process through normal HTTP handling
		p.handleHTTP(rw, req)

		// Check if we should close the connection
		if req.Close || rw.closeAfter {
			return
		}
	}
}

// connResponseWriter implements http.ResponseWriter for raw connections.
type connResponseWriter struct {
	conn        net.Conn
	headers     http.Header
	wroteHeader bool
	statusCode  int
	closeAfter  bool
}

func (w *connResponseWriter) Header() http.Header {
	return w.headers
}

func (w *connResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.statusCode = statusCode

	// Write status line
	statusText := http.StatusText(statusCode)
	fmt.Fprintf(w.conn, "HTTP/1.1 %d %s\r\n", statusCode, statusText)

	// Write headers
	for key, values := range w.headers {
		for _, value := range values {
			fmt.Fprintf(w.conn, "%s: %s\r\n", key, value)
		}
	}

	// Check Connection header
	if w.headers.Get("Connection") == "close" {
		w.closeAfter = true
	}

	// End headers
	fmt.Fprint(w.conn, "\r\n")
}

func (w *connResponseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.conn.Write(data)
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
