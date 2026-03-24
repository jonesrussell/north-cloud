package metrics_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/publisher/internal/metrics"
	"github.com/stretchr/testify/assert"
)

func TestRedisKeys_Posted(t *testing.T) {
	t.Helper()

	keys := metrics.NewRedisKeys("metrics")

	assert.Equal(t, "metrics:posted:thunder_bay", keys.Posted("thunder_bay"))
	assert.Equal(t, "metrics:posted:ottawa", keys.Posted("ottawa"))
}

func TestRedisKeys_Skipped(t *testing.T) {
	t.Helper()

	keys := metrics.NewRedisKeys("metrics")

	assert.Equal(t, "metrics:skipped:thunder_bay", keys.Skipped("thunder_bay"))
	assert.Equal(t, "metrics:skipped:ottawa", keys.Skipped("ottawa"))
}

func TestRedisKeys_Errors(t *testing.T) {
	t.Helper()

	keys := metrics.NewRedisKeys("metrics")

	assert.Equal(t, "metrics:errors:thunder_bay", keys.Errors("thunder_bay"))
	assert.Equal(t, "metrics:errors:ottawa", keys.Errors("ottawa"))
}

func TestRedisKeys_CustomPrefix(t *testing.T) {
	t.Helper()

	keys := metrics.NewRedisKeys("custom")

	assert.Equal(t, "custom:posted:city", keys.Posted("city"))
	assert.Equal(t, "custom:skipped:city", keys.Skipped("city"))
	assert.Equal(t, "custom:errors:city", keys.Errors("city"))
}

func TestKeyConstants(t *testing.T) {
	t.Helper()

	assert.Equal(t, "metrics", metrics.KeyPrefixMetrics)
	assert.Equal(t, "posted", metrics.KeyPrefixPosted)
	assert.Equal(t, "skipped", metrics.KeyPrefixSkipped)
	assert.Equal(t, "errors", metrics.KeyPrefixErrors)
	assert.Equal(t, "metrics:recent:items", metrics.KeyRecentItems)
	assert.Equal(t, "metrics:last_sync", metrics.KeyLastSync)
	assert.Equal(t, 100, metrics.MaxRecentItems)
	assert.Equal(t, 30, metrics.MetricsTTLDays)
	assert.Equal(t, 7, metrics.RecentItemsTTLDays)
	assert.Equal(t, 24, metrics.HoursPerDay)
}
