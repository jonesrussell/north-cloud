package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeStats(t *testing.T) {
	t.Parallel()

	docs := []map[string]any{
		{
			"url":           "https://example.com/article-1",
			"title":         "Test Article",
			"content":       "This is a test article with some words",
			"indexed_at":    "2026-03-10T12:00:00Z",
			"quality_score": 0.85,
		},
		{
			"url":     "https://example.com/article-2",
			"title":   "Empty Content",
			"content": "",
		},
	}

	stats := computeStats(docs)

	require.Len(t, stats, 2)

	assert.Equal(t, "https://example.com/article-1", stats[0].URL)
	assert.Equal(t, "Test Article", stats[0].Title)
	assert.Equal(t, 8, stats[0].WordCount)
	assert.Equal(t, len("This is a test article with some words"), stats[0].ContentLength)
	assert.Equal(t, "2026-03-10T12:00:00Z", stats[0].IndexedAt)
	assert.InDelta(t, 0.85, stats[0].QualityScore, 0.001)

	assert.Equal(t, 0, stats[1].WordCount)
	assert.Equal(t, 0, stats[1].ContentLength)
}

func TestComputeStatsEmpty(t *testing.T) {
	t.Parallel()

	stats := computeStats(nil)
	assert.Empty(t, stats)
}

func TestWriteTable(t *testing.T) {
	t.Parallel()

	stats := []DocStats{
		{
			URL:           "https://example.com/a",
			Title:         "Test",
			WordCount:     100,
			ContentLength: 500,
			QualityScore:  0.90,
			IndexedAt:     "2026-03-10",
		},
	}

	var buf bytes.Buffer
	writeTable(&buf, stats)

	output := buf.String()
	assert.Contains(t, output, "URL")
	assert.Contains(t, output, "WORDS")
	assert.Contains(t, output, "https://example.com/a")
	assert.Contains(t, output, "100")
}

func TestWriteTableTruncatesLongTitles(t *testing.T) {
	t.Parallel()

	stats := []DocStats{
		{
			Title: "This is a very long title that exceeds the maximum display length",
		},
	}

	var buf bytes.Buffer
	writeTable(&buf, stats)

	output := buf.String()
	assert.Contains(t, output, "...")
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	stats := []DocStats{
		{
			URL:           "https://example.com/a",
			Title:         "Test",
			WordCount:     100,
			ContentLength: 500,
			QualityScore:  0.90,
			IndexedAt:     "2026-03-10",
		},
	}

	var buf bytes.Buffer
	err := writeJSON(&buf, stats)
	require.NoError(t, err)

	var decoded []DocStats
	decodeErr := json.Unmarshal(buf.Bytes(), &decoded)
	require.NoError(t, decodeErr)
	require.Len(t, decoded, 1)
	assert.Equal(t, "https://example.com/a", decoded[0].URL)
}

func TestWriteJSONEmpty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := writeJSON(&buf, []DocStats{})
	require.NoError(t, err)

	output := strings.TrimSpace(buf.String())
	assert.Equal(t, "[]", output)
}
