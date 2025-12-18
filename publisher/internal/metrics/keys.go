package metrics

import "fmt"

const (
	// KeyPrefixMetrics is the prefix for all metrics keys
	KeyPrefixMetrics = "metrics"
	// KeyPrefixPosted is the prefix for posted counters
	KeyPrefixPosted = "posted"
	// KeyPrefixSkipped is the prefix for skipped counters
	KeyPrefixSkipped = "skipped"
	// KeyPrefixErrors is the prefix for error counters
	KeyPrefixErrors = "errors"
	// KeyRecentArticles is the Redis key for recent articles list
	KeyRecentArticles = "metrics:recent:articles"
	// KeyLastSync is the Redis key for last sync timestamp
	KeyLastSync = "metrics:last_sync"
	// MaxRecentArticles is the maximum number of recent articles to keep
	MaxRecentArticles = 100
	// MetricsTTLDays is the TTL in days for metrics counters
	MetricsTTLDays = 30
	// RecentArticlesTTLDays is the TTL in days for recent articles list
	RecentArticlesTTLDays = 7
)

// RedisKeys provides methods to build Redis keys consistently
type RedisKeys struct {
	prefix string
}

// NewRedisKeys creates a new RedisKeys instance
func NewRedisKeys(prefix string) *RedisKeys {
	return &RedisKeys{prefix: prefix}
}

// Posted returns the Redis key for posted counter for a city
func (k *RedisKeys) Posted(city string) string {
	return fmt.Sprintf("%s:%s:%s", k.prefix, KeyPrefixPosted, city)
}

// Skipped returns the Redis key for skipped counter for a city
func (k *RedisKeys) Skipped(city string) string {
	return fmt.Sprintf("%s:%s:%s", k.prefix, KeyPrefixSkipped, city)
}

// Errors returns the Redis key for error counter for a city
func (k *RedisKeys) Errors(city string) string {
	return fmt.Sprintf("%s:%s:%s", k.prefix, KeyPrefixErrors, city)
}
