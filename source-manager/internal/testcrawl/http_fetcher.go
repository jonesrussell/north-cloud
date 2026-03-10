package testcrawl

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// httpFetchTimeout is the maximum time to wait for an HTTP response.
const httpFetchTimeout = 30 * time.Second

// httpDialTimeout is the timeout for establishing TCP connections.
const httpDialTimeout = 10 * time.Second

// httpMaxIdleConns is the maximum number of idle connections.
const httpMaxIdleConns = 10

// httpMaxRedirects is the maximum number of redirects to follow.
const httpMaxRedirects = 10

// httpFetcher is an SSRF-safe HTTP client for fetching pages in the test-crawl pipeline.
type httpFetcher struct {
	client *http.Client
}

// newHTTPFetcher creates an httpFetcher with SSRF protection (blocks private IPs at dial time).
func newHTTPFetcher() *httpFetcher {
	transport := &http.Transport{
		DialContext:           safeDialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          httpMaxIdleConns,
		IdleConnTimeout:       httpFetchTimeout,
		TLSHandshakeTimeout:   httpDialTimeout,
		ExpectContinueTimeout: 1 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   httpFetchTimeout,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= httpMaxRedirects {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
	return &httpFetcher{client: client}
}

// Fetch performs a GET request and returns the response body as a string.
func (f *httpFetcher) Fetch(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; North-Cloud-SourceManager/1.0; +test-crawl)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}

	// Read body with a size cap to prevent runaway allocations (10 MB).
	const maxBodyBytes = 10 * 1024 * 1024
	const readChunkBytes = 32 * 1024
	buf := make([]byte, 0, maxBodyBytes)
	tmp := make([]byte, readChunkBytes)
	for {
		n, readErr := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if readErr != nil {
			break
		}
		if len(buf) >= maxBodyBytes {
			break
		}
	}
	return string(buf), nil
}

// safeDialContext blocks connections to private/loopback IP addresses (SSRF protection).
func safeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("DNS resolution failed: %w", err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no IP addresses found for host: %s", host)
	}

	for _, ipAddr := range ips {
		if isPrivateOrReservedIP(ipAddr.IP) {
			return nil, fmt.Errorf("connection to private/internal IP address is not allowed: %s resolves to %s", host, ipAddr.IP)
		}
	}

	dialer := &net.Dialer{Timeout: httpDialTimeout}
	target := net.JoinHostPort(ips[0].IP.String(), port)
	return dialer.DialContext(ctx, network, target)
}

// isPrivateOrReservedIP returns true for IPs that should never be dialled from a web service.
func isPrivateOrReservedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
