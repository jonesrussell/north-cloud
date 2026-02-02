package discovery_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/publisher/internal/discovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterClassifiedIndexes(t *testing.T) {
	t.Helper()

	testCases := []struct {
		name     string
		response []map[string]string
		expected []string
	}{
		{
			name: "filters classified content indexes",
			response: []map[string]string{
				{"index": "www_sudbury_com_classified_content"},
				{"index": "www_baytoday_ca_classified_content"},
				{"index": ".kibana_1"},
				{"index": "www_sudbury_com_raw_content"},
			},
			expected: []string{
				"www_sudbury_com_classified_content",
				"www_baytoday_ca_classified_content",
			},
		},
		{
			name: "filters out system indexes",
			response: []map[string]string{
				{"index": ".kibana"},
				{"index": ".security"},
				{"index": "test_classified_content"},
			},
			expected: []string{
				"test_classified_content",
			},
		},
		{
			name:     "handles empty response",
			response: []map[string]string{},
			expected: []string{},
		},
		{
			name: "handles missing index key",
			response: []map[string]string{
				{"health": "green", "status": "open"},
				{"index": "valid_classified_content"},
			},
			expected: []string{
				"valid_classified_content",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			indexes := discovery.FilterClassifiedIndexes(tc.response)

			require.Len(t, indexes, len(tc.expected))
			for _, expected := range tc.expected {
				assert.Contains(t, indexes, expected)
			}
		})
	}
}
