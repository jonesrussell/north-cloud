package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractionLoss(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		liveWords  int
		storedWords int
		expected   float64
	}{
		{
			name:        "no loss",
			liveWords:   100,
			storedWords: 100,
			expected:    0,
		},
		{
			name:        "50 percent loss",
			liveWords:   200,
			storedWords: 100,
			expected:    50.0,
		},
		{
			name:        "zero live words",
			liveWords:   0,
			storedWords: 100,
			expected:    0,
		},
		{
			name:        "both zero",
			liveWords:   0,
			storedWords: 0,
			expected:    0,
		},
		{
			name:        "stored exceeds live",
			liveWords:   50,
			storedWords: 100,
			expected:    -100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := extractionLoss(tt.liveWords, tt.storedWords)
			assert.InDelta(t, tt.expected, result, 0.01)
		})
	}
}
