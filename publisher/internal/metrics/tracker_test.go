//nolint:testpackage // Testing unexported conversion functions requires same package access
package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToRecentItem_FromRecentItem(t *testing.T) {
	t.Helper()

	now := time.Now()
	input := RecentItem{
		ID:       "abc-123",
		Title:    "Test Article",
		URL:      "https://example.com/test",
		City:     "Thunder Bay",
		PostedAt: now,
	}

	result, err := convertToRecentItem(input)
	require.NoError(t, err)
	assert.Equal(t, input.ID, result.ID)
	assert.Equal(t, input.Title, result.Title)
	assert.Equal(t, input.URL, result.URL)
	assert.Equal(t, input.City, result.City)
	assert.Equal(t, input.PostedAt, result.PostedAt)
}

func TestConvertToRecentItem_FromMap(t *testing.T) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	input := map[string]any{
		"id":        "abc-123",
		"title":     "Test Article",
		"url":       "https://example.com/test",
		"city":      "Thunder Bay",
		"posted_at": now.Format(time.RFC3339),
	}

	result, err := convertToRecentItem(input)
	require.NoError(t, err)
	assert.Equal(t, "abc-123", result.ID)
	assert.Equal(t, "Test Article", result.Title)
	assert.Equal(t, "https://example.com/test", result.URL)
	assert.Equal(t, "Thunder Bay", result.City)
	assert.Equal(t, now, result.PostedAt)
}

func TestConvertToRecentItem_FromMapMissingFields(t *testing.T) {
	t.Helper()

	input := map[string]any{
		"id":    "abc-123",
		"title": "Test Article",
	}

	result, err := convertToRecentItem(input)
	require.NoError(t, err)
	assert.Equal(t, "abc-123", result.ID)
	assert.Equal(t, "Test Article", result.Title)
	assert.Empty(t, result.URL)
	assert.Empty(t, result.City)
	// PostedAt should default to approximately now
	assert.WithinDuration(t, time.Now(), result.PostedAt, 2*time.Second)
}

func TestConvertToRecentItem_FromMapInvalidPostedAt(t *testing.T) {
	t.Helper()

	input := map[string]any{
		"id":        "abc-123",
		"posted_at": "not-a-date",
	}

	result, err := convertToRecentItem(input)
	require.NoError(t, err)
	assert.Equal(t, "abc-123", result.ID)
	// PostedAt should default to approximately now when parsing fails
	assert.WithinDuration(t, time.Now(), result.PostedAt, 2*time.Second)
}

func TestConvertToRecentItem_FromStruct(t *testing.T) {
	t.Helper()

	type custom struct {
		ID       string    `json:"id"`
		Title    string    `json:"title"`
		URL      string    `json:"url"`
		City     string    `json:"city"`
		PostedAt time.Time `json:"posted_at"`
	}

	now := time.Now().UTC().Truncate(time.Second)
	input := custom{
		ID:       "abc-123",
		Title:    "Test Article",
		URL:      "https://example.com/test",
		City:     "Thunder Bay",
		PostedAt: now,
	}

	result, err := convertToRecentItem(input)
	require.NoError(t, err)
	assert.Equal(t, "abc-123", result.ID)
	assert.Equal(t, "Test Article", result.Title)
}

func TestConvertMapToRecentItem_AllFields(t *testing.T) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	input := map[string]any{
		"id":        "abc-123",
		"title":     "Test Title",
		"url":       "https://example.com",
		"city":      "Ottawa",
		"posted_at": now.Format(time.RFC3339),
	}

	result := convertMapToRecentItem(input)
	assert.Equal(t, "abc-123", result.ID)
	assert.Equal(t, "Test Title", result.Title)
	assert.Equal(t, "https://example.com", result.URL)
	assert.Equal(t, "Ottawa", result.City)
	assert.Equal(t, now, result.PostedAt)
}

func TestConvertMapToRecentItem_EmptyMap(t *testing.T) {
	t.Helper()

	input := map[string]any{}

	result := convertMapToRecentItem(input)
	assert.Empty(t, result.ID)
	assert.Empty(t, result.Title)
	assert.Empty(t, result.URL)
	assert.Empty(t, result.City)
	// PostedAt should default to approximately now
	assert.WithinDuration(t, time.Now(), result.PostedAt, 2*time.Second)
}

func TestConvertMapToRecentItem_WrongTypes(t *testing.T) {
	t.Helper()

	input := map[string]any{
		"id":    42,      // int, not string
		"title": true,    // bool, not string
		"url":   []int{}, // slice, not string
	}

	result := convertMapToRecentItem(input)
	// Non-string types should be ignored, fields remain zero values
	assert.Empty(t, result.ID)
	assert.Empty(t, result.Title)
	assert.Empty(t, result.URL)
}

func TestConvertViaJSON_ValidStruct(t *testing.T) {
	t.Helper()

	type input struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}

	result, err := convertViaJSON(input{ID: "x", Title: "y"})
	require.NoError(t, err)
	assert.Equal(t, "x", result.ID)
	assert.Equal(t, "y", result.Title)
}

func TestConvertViaJSON_Unmarshallable(t *testing.T) {
	t.Helper()

	// Channels cannot be marshaled to JSON
	_, err := convertViaJSON(make(chan int))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal item")
}

func TestNewTracker(t *testing.T) {
	t.Helper()

	// NewTracker should not panic with nil client (used for construction only)
	// We just verify it creates the struct correctly
	cities := []string{"thunder_bay", "ottawa"}
	tracker := NewTracker(nil, cities, nil)

	require.NotNil(t, tracker)
	assert.Equal(t, cities, tracker.cities)
	assert.NotNil(t, tracker.keys)
}

func TestRecentItem_Fields(t *testing.T) {
	t.Helper()

	now := time.Now()
	item := RecentItem{
		ID:       "test-id",
		Title:    "Test Title",
		URL:      "https://example.com",
		City:     "Ottawa",
		PostedAt: now,
	}

	assert.Equal(t, "test-id", item.ID)
	assert.Equal(t, "Test Title", item.Title)
	assert.Equal(t, "https://example.com", item.URL)
	assert.Equal(t, "Ottawa", item.City)
	assert.Equal(t, now, item.PostedAt)
}

func TestStats_Fields(t *testing.T) {
	t.Helper()

	stats := Stats{
		TotalPosted:  100,
		TotalSkipped: 20,
		TotalErrors:  5,
		Cities: []CityStats{
			{Name: "thunder_bay", Posted: 60, Skipped: 10, Errors: 3},
			{Name: "ottawa", Posted: 40, Skipped: 10, Errors: 2},
		},
	}

	assert.Equal(t, int64(100), stats.TotalPosted)
	assert.Equal(t, int64(20), stats.TotalSkipped)
	assert.Equal(t, int64(5), stats.TotalErrors)
	require.Len(t, stats.Cities, 2)
	assert.Equal(t, "thunder_bay", stats.Cities[0].Name)
	assert.Equal(t, int64(60), stats.Cities[0].Posted)
}
