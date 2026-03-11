package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildQuery(t *testing.T) {
	t.Parallel()

	const testLimit = 10

	query := buildQuery("test-source", testLimit)

	size, ok := query["size"].(int)
	require.True(t, ok, "size should be an int")
	assert.Equal(t, testLimit, size)

	queryField, ok := query["query"].(map[string]any)
	require.True(t, ok, "query should be a map")

	term, ok := queryField["term"].(map[string]any)
	require.True(t, ok, "term should be a map")

	sourceFilter, ok := term["source_name.keyword"].(string)
	require.True(t, ok, "source_name.keyword should be a string")
	assert.Equal(t, "test-source", sourceFilter)

	sortField, ok := query["sort"].([]map[string]any)
	require.True(t, ok, "sort should be a slice of maps")
	require.Len(t, sortField, 1)

	_, hasIndexedAt := sortField[0]["indexed_at"]
	assert.True(t, hasIndexedAt, "sort should contain indexed_at")
}

func TestCountWords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{name: "empty string", input: "", expected: 0},
		{name: "single word", input: "hello", expected: 1},
		{name: "multiple words", input: "hello world foo bar", expected: 4},
		{name: "extra whitespace", input: "  hello   world  ", expected: 2},
		{name: "tabs and newlines", input: "hello\tworld\nfoo", expected: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, countWords(tt.input))
		})
	}
}
