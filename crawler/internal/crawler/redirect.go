package crawler

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
)

const (
	redirectCheckTimeout       = 10 * time.Second
	redirectStatusMovedPerm    = http.StatusMovedPermanently  // 301
	redirectStatusFound        = http.StatusFound             // 302
	redirectStatusSeeOther     = http.StatusSeeOther          // 303
	redirectStatusTempRedirect = http.StatusTemporaryRedirect // 307
	redirectStatusPermRedirect = http.StatusPermanentRedirect // 308
)

// checkRedirect issues a HEAD request to the source URL with redirect-following
// disabled. If the response is a redirect to a different host (not in
// AllowedDomains), it returns an error so the crawl is aborted early.
// Connection errors are non-fatal: logged as warnings but do not abort.
func (c *Crawler) checkRedirect(ctx context.Context, source *configtypes.Source) error {
	client := &http.Client{
		Timeout: redirectCheckTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse // do not follow redirects
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, source.URL, http.NoBody)
	if err != nil {
		return fmt.Errorf("redirect check: build request: %w", err)
	}

	resp, err := client.Do(req) //nolint:gosec // G704: URL is from source config
	if err != nil {
		// Connection errors are non-fatal — warn and let the crawl proceed.
		c.GetJobLogger().Warn(logs.CategoryLifecycle,
			"Pre-crawl redirect check failed (non-fatal)",
			logs.URL(source.URL),
			logs.Err(err),
		)
		return nil
	}
	defer resp.Body.Close()

	if !isRedirectStatus(resp.StatusCode) {
		return nil
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return nil
	}

	return handlePossibleDomainRedirect(source, location)
}

// isRedirectStatus returns true for HTTP status codes that indicate a redirect.
func isRedirectStatus(code int) bool {
	switch code {
	case redirectStatusMovedPerm,
		redirectStatusFound,
		redirectStatusSeeOther,
		redirectStatusTempRedirect,
		redirectStatusPermRedirect:
		return true
	default:
		return false
	}
}

// handlePossibleDomainRedirect compares the redirect Location host against the
// source's AllowedDomains. If the redirect host is not in AllowedDomains, it
// returns an error indicating a cross-domain redirect.
func handlePossibleDomainRedirect(source *configtypes.Source, location string) error {
	parsed, err := url.Parse(location)
	if err != nil {
		return fmt.Errorf("redirect check: parse Location %q: %w", location, err)
	}

	redirectHost := parsed.Hostname()
	if redirectHost == "" {
		// Relative redirect — same domain, no issue.
		return nil
	}

	for _, allowed := range source.AllowedDomains {
		if hostsMatch(redirectHost, allowed) {
			return nil
		}
	}

	return fmt.Errorf(
		"source %q redirects to %q (host %q) which is not in AllowedDomains %v",
		source.URL, location, redirectHost, source.AllowedDomains,
	)
}

// hostsMatch compares two hostnames, stripping port numbers if present.
func hostsMatch(a, b string) bool {
	return stripPort(a) == stripPort(b)
}

// stripPort removes the port portion from a host string.
func stripPort(host string) string {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		return host // no port present
	}
	return h
}
