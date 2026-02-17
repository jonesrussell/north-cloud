package crawler

import (
	"context"
	"regexp"

	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
)

// Test exports for internal functions.

// IsArticleURL exports isArticleURL for testing.
var IsArticleURL = isArticleURL

// IsArticlePage exports isArticlePage for testing.
var IsArticlePage = isArticlePage

// CompileArticlePatterns exports compileArticlePatterns for testing.
var CompileArticlePatterns = compileArticlePatterns

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
	return compileArticlePatterns(patterns)
}
