package crawler

import (
	"context"
	"regexp"

	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
)

// Test exports for internal functions.

// IsContentURL exports isContentURL for testing.
var IsContentURL = isContentURL

// IsContentPage exports isContentPage for testing.
var IsContentPage = isContentPage

// CompileContentPatterns exports compileContentPatterns for testing.
var CompileContentPatterns = compileContentPatterns

// HasNewsArticleJSONLD exports hasNewsArticleJSONLD for testing.
var HasNewsArticleJSONLD = hasNewsArticleJSONLD

// HasStructuredContentJSONLD exports hasStructuredContentJSONLD for testing.
var HasStructuredContentJSONLD = hasStructuredContentJSONLD

// DetectContentTypeFromURL exports detectContentTypeFromURL for testing.
var DetectContentTypeFromURL = detectContentTypeFromURL

// DetectContentTypeFromHTML exports detectContentTypeFromHTML for testing.
var DetectContentTypeFromHTML = detectContentTypeFromHTML

// IsRedirectStatus exports isRedirectStatus for testing.
var IsRedirectStatus = isRedirectStatus

// HandlePossibleDomainRedirect exports handlePossibleDomainRedirect for testing.
var HandlePossibleDomainRedirect = handlePossibleDomainRedirect

// HostsMatch exports hostsMatch for testing.
var HostsMatch = hostsMatch

// CheckRedirect exports checkRedirect via a zero-value Crawler for testing.
func CheckRedirect(ctx context.Context, source *configtypes.Source) error {
	c := &Crawler{}
	return c.checkRedirect(ctx, source)
}

// ExplicitPatterns is a helper to build explicit pattern slices for tests.
func ExplicitPatterns(patterns ...string) []*regexp.Regexp {
	return compileContentPatterns(patterns)
}
