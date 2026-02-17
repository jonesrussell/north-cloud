// Package frontier provides URL normalization and hashing for the URL frontier queue.
// URLs are normalized before insertion so that the same URL expressed differently
// produces the same hash for deduplication.
package frontier

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
)

// trackingParams lists query parameters that are stripped during normalization.
// These are advertising and analytics trackers that do not affect page content.
var trackingParams = map[string]struct{}{
	"utm_source":   {},
	"utm_medium":   {},
	"utm_campaign": {},
	"utm_term":     {},
	"utm_content":  {},
	"fbclid":       {},
	"gclid":        {},
	"gclsrc":       {},
	"dclid":        {},
	"msclkid":      {},
}

// defaultPorts maps schemes to their default port strings.
var defaultPorts = map[string]string{
	"http":  "80",
	"https": "443",
}

var (
	errEmptyInput          = errors.New("normalize url: empty input")
	errMissingSchemeOrHost = errors.New("normalize url: missing scheme or host")
	errEmptyHostInput      = errors.New("extract host: empty input")
)

// NormalizeURL applies deterministic transformations to a raw URL so that
// equivalent URLs produce identical strings. Transformations include lowercasing
// scheme and host, upgrading http to https, removing default ports, resolving
// path dot-segments, removing trailing slashes, removing fragments, sorting
// query parameters, and stripping tracking parameters.
func NormalizeURL(rawURL string) (string, error) {
	if rawURL == "" {
		return "", errEmptyInput
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("normalize url: %w", err)
	}

	if validateErr := validateParsedURL(parsed); validateErr != nil {
		return "", validateErr
	}

	originalScheme := strings.ToLower(parsed.Scheme)
	parsed.Scheme = "https"
	parsed.Host = normalizeHost(parsed, originalScheme)
	parsed.Fragment = ""
	parsed.RawQuery = buildCleanQuery(parsed.Query())
	parsed.Path = normalizePath(parsed.Path)

	return parsed.String(), nil
}

// URLHash normalizes the given URL and returns its SHA-256 hex digest.
// The returned string is always 64 characters long (SHA-256 hex encoding).
func URLHash(rawURL string) (string, error) {
	normalized, err := NormalizeURL(rawURL)
	if err != nil {
		return "", fmt.Errorf("url hash: %w", err)
	}

	sum := sha256.Sum256([]byte(normalized))

	return hex.EncodeToString(sum[:]), nil
}

// ExtractHost returns the hostname (without port) from a URL, lowercased.
func ExtractHost(rawURL string) (string, error) {
	if rawURL == "" {
		return "", errEmptyHostInput
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("extract host: %w", err)
	}

	if validateErr := validateParsedURL(parsed); validateErr != nil {
		return "", validateErr
	}

	return strings.ToLower(parsed.Hostname()), nil
}

// validateParsedURL checks that a parsed URL has the minimum required components.
func validateParsedURL(u *url.URL) error {
	if u.Scheme == "" || u.Host == "" {
		return errMissingSchemeOrHost
	}

	return nil
}

// normalizeHost lowercases the hostname and removes default ports.
// originalScheme is the scheme before upgrade to https, used to identify
// default ports (e.g., port 80 is default for http).
func normalizeHost(u *url.URL, originalScheme string) string {
	hostname := strings.ToLower(u.Hostname())
	port := u.Port()

	if port == "" {
		return hostname
	}

	// Remove port if it matches the default for either the original or final scheme.
	for _, scheme := range []string{originalScheme, u.Scheme} {
		if defaultPort, ok := defaultPorts[scheme]; ok && port == defaultPort {
			return hostname
		}
	}

	return hostname + ":" + port
}

// buildCleanQuery strips tracking parameters, sorts the remaining keys
// alphabetically, and returns the encoded query string. Returns an empty
// string when no parameters remain after filtering.
func buildCleanQuery(values url.Values) string {
	keys := make([]string, 0, len(values))

	for key := range values {
		if _, isTracking := trackingParams[key]; !isTracking {
			keys = append(keys, key)
		}
	}

	if len(keys) == 0 {
		return ""
	}

	sort.Strings(keys)

	var b strings.Builder

	for i, key := range keys {
		if i > 0 {
			b.WriteByte('&')
		}

		vals := values[key]
		for j, val := range vals {
			if j > 0 {
				b.WriteByte('&')
			}

			b.WriteString(url.QueryEscape(key))
			b.WriteByte('=')
			b.WriteString(url.QueryEscape(val))
		}
	}

	return b.String()
}

// normalizePath resolves dot-segments (/../, /./) and removes trailing slashes
// while preserving the root "/".
func normalizePath(p string) string {
	if p == "" || p == "/" {
		return "/"
	}

	cleaned := path.Clean(p)

	return strings.TrimRight(cleaned, "/")
}
