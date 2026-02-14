package crawler

import "regexp"

// Test exports for internal functions.

// IsArticleURL exports isArticleURL for testing.
var IsArticleURL = isArticleURL

// IsArticlePage exports isArticlePage for testing.
var IsArticlePage = isArticlePage

// CompileArticlePatterns exports compileArticlePatterns for testing.
var CompileArticlePatterns = compileArticlePatterns

// ExplicitPatterns is a helper to build explicit pattern slices for tests.
func ExplicitPatterns(patterns ...string) []*regexp.Regexp {
	return compileArticlePatterns(patterns)
}
