// Package discovery provides the Source Identity Resolver and Source Candidate Pipeline
// for automatic source discovery. Identity is not equal to hostname (e.g. Medium, Substack
// have many logical sources per host). All resolution decisions are deterministic and logged.
package discovery

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/jonesrussell/north-cloud/crawler/internal/frontier"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/apiclient"
)

const (
	platformSubstack = "substack"
	platformMedium   = "medium"
)

// ResolvedKind is the outcome of resolving a discovered URL.
type ResolvedKind string

const (
	// ResolvedExisting means the URL belongs to an existing source (use returned source_id for frontier).
	ResolvedExisting ResolvedKind = "existing"
	// ResolvedNew means the URL is a candidate for a new logical source.
	ResolvedNew ResolvedKind = "new"
	// ResolvedPlatformSub means the URL belongs to a known platform; identity is platform:tenant (candidate).
	ResolvedPlatformSub ResolvedKind = "platform_sub"
)

// ResolvedIdentity is the result of the Source Identity Resolver.
type ResolvedIdentity struct {
	Kind        ResolvedKind
	SourceID    string // set when Kind == ResolvedExisting
	IdentityKey string // always set; the derived identity key for this URL
	Reason      string // explicit reason for audit log
}

// IdentityResolver resolves a discovered URL to an existing source or a new/platform candidate.
type IdentityResolver struct {
	client *apiclient.Client
	log    Logger
}

// Logger is the interface for logging pipeline decisions at various severity levels.
type Logger interface {
	Info(msg string, keysAndValues ...any)
	Warn(msg string, keysAndValues ...any)
	Error(msg string, keysAndValues ...any)
}

// NewIdentityResolver creates a new Source Identity Resolver.
func NewIdentityResolver(client *apiclient.Client, log Logger) *IdentityResolver {
	return &IdentityResolver{client: client, log: log}
}

// Resolve determines whether the discovered URL belongs to an existing source, a new source, or a platform sub-source.
// CanonicalURL should be the normalized URL (use frontier.NormalizeURL). ReferringSourceID is optional context.
// All decisions are deterministic and logged with an explicit reason.
func (r *IdentityResolver) Resolve(ctx context.Context, canonicalURL, referringSourceID string) (*ResolvedIdentity, error) {
	if canonicalURL == "" {
		return nil, errors.New("resolve: empty canonical URL")
	}

	identityKey, keyReason, err := deriveIdentityKey(canonicalURL)
	if err != nil {
		return nil, fmt.Errorf("derive identity key: %w", err)
	}

	source, err := r.client.GetByIdentityKey(ctx, identityKey)
	if err != nil {
		return nil, fmt.Errorf("lookup by identity_key: %w", err)
	}

	if source != nil {
		r.log.Info("Source Identity Resolver: matched existing source",
			"url", canonicalURL,
			"identity_key", identityKey,
			"source_id", source.ID,
			"reason", "identity_key lookup hit",
		)
		return &ResolvedIdentity{
			Kind:        ResolvedExisting,
			SourceID:    source.ID,
			IdentityKey: identityKey,
			Reason:      "matched existing source_id " + source.ID + " by identity_key",
		}, nil
	}

	// No existing source: decide if platform sub-source or plain new source
	platformID := detectPlatform(canonicalURL)
	if platformID != "" {
		r.log.Info("Source Identity Resolver: platform sub-source candidate",
			"url", canonicalURL,
			"identity_key", identityKey,
			"platform", platformID,
			"reason", keyReason,
		)
		return &ResolvedIdentity{
			Kind:        ResolvedPlatformSub,
			IdentityKey: identityKey,
			Reason:      "platform sub-source candidate, identity_key=" + identityKey,
		}, nil
	}

	r.log.Info("Source Identity Resolver: new source candidate",
		"url", canonicalURL,
		"identity_key", identityKey,
		"reason", keyReason,
	)
	return &ResolvedIdentity{
		Kind:        ResolvedNew,
		IdentityKey: identityKey,
		Reason:      "new source candidate, identity_key=" + identityKey,
	}, nil
}

// deriveIdentityKey returns a deterministic identity key from a canonical URL.
// Default: host (lowercase) so one logical source per host unless platform rules apply.
// Platform rules (Substack, Medium, etc.) use "platform:tenant" from path.
func deriveIdentityKey(canonicalURL string) (identityKey, reason string, err error) {
	parsed, err := url.Parse(canonicalURL)
	if err != nil {
		return "", "", fmt.Errorf("parse URL: %w", err)
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", "", errors.New("empty host")
	}

	// Platform-specific extraction (design: platform registry with path conventions)
	if platform, tenant := extractPlatformIdentity(host, parsed.Path); platform != "" {
		key := platform + ":" + tenant
		return key, "platform " + platform + " tenant from path", nil
	}

	// Default: identity = host (one logical source per host)
	return host, "identity_key=host (default)", nil
}

// extractPlatformIdentity returns (platformID, tenantOrAuthor) for known platforms.
// Currently recognizes "substack.com" and "medium.com" by exact host match.
// Note: subdomain-based Substack blogs (e.g. example.substack.com) are NOT matched;
// they fall through to the default host-based identity key.
func extractPlatformIdentity(host, rawPath string) (platform, tenant string) {
	pathClean := path.Clean(rawPath)
	pathClean = strings.Trim(pathClean, "/")
	segments := strings.Split(pathClean, "/")

	switch host {
	case "substack.com":
		if len(segments) >= 1 && segments[0] != "" {
			return platformSubstack, segments[0]
		}
		return platformSubstack, ""
	case "medium.com":
		if len(segments) >= 1 && strings.HasPrefix(segments[0], "@") {
			return platformMedium, strings.TrimPrefix(segments[0], "@")
		}
		return platformMedium, ""
	default:
		return "", ""
	}
}

// detectPlatform returns a short platform id if the URL is on a known multi-tenant platform.
func detectPlatform(canonicalURL string) string {
	parsed, err := url.Parse(canonicalURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	platform, _ := extractPlatformIdentity(host, parsed.Path)
	return platform
}

// CanonicalizeURL normalizes a raw URL for resolver and pipeline (scheme, host, path, strip tracking).
// Uses frontier.NormalizeURL for consistency with the rest of the crawler.
func CanonicalizeURL(rawURL string) (string, error) {
	return frontier.NormalizeURL(rawURL)
}
