package integration

import "errors"

// Error types for the integration package.
var (
	// ErrElasticsearchQuery is returned when an Elasticsearch query fails
	ErrElasticsearchQuery = errors.New("elasticsearch query failed")

	// ErrDrupalPostFailed is returned when posting to Drupal fails
	ErrDrupalPostFailed = errors.New("drupal post failed")

	// ErrArticleAlreadyPosted is returned when an article has already been posted
	ErrArticleAlreadyPosted = errors.New("article already posted")

	// ErrRateLimitExceeded is returned when rate limit is exceeded
	ErrRateLimitExceeded = errors.New("rate limit exceeded")

	// ErrArticleNotFound is returned when an article is not found
	ErrArticleNotFound = errors.New("article not found")

	// ErrInvalidCityConfig is returned when city configuration is invalid
	ErrInvalidCityConfig = errors.New("invalid city configuration")
)
