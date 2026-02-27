//nolint:testpackage // Testing with unexported constants qualityWeightOK, sourceCountCap, etc.
package domain

import (
	"testing"
	"time"
)

// halfDecayDays is the approximate midpoint of the recency decay window.
const halfDecayDays = 15

func floatPtr(f float64) *float64 {
	return &f
}

func TestComputeQualityScore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		agg     DomainAggregate
		wantMin int
		wantMax int
	}{
		{
			name: "nil ratios and old date yields zero",
			agg: DomainAggregate{
				OKRatio:     nil,
				HTMLRatio:   nil,
				SourceCount: 0,
				LastSeen:    time.Now().Add(-time.Hour * hoursPerDay * recencyDecayDays * 2),
			},
			wantMin: 0,
			wantMax: 0,
		},
		{
			name: "perfect scores yield 99-100",
			agg: DomainAggregate{
				OKRatio:     floatPtr(1.0),
				HTMLRatio:   floatPtr(1.0),
				SourceCount: sourceCountCap,
				LastSeen:    time.Now(),
			},
			wantMin: 99,
			wantMax: maxQualityScore,
		},
		{
			name: "only ok ratio contributes 30",
			agg: DomainAggregate{
				OKRatio:     floatPtr(1.0),
				HTMLRatio:   nil,
				SourceCount: 0,
				LastSeen:    time.Now().Add(-time.Hour * hoursPerDay * recencyDecayDays * 2),
			},
			wantMin: qualityWeightOK,
			wantMax: qualityWeightOK,
		},
		{
			name: "source cap at 5 contributes 20",
			agg: DomainAggregate{
				OKRatio:     nil,
				HTMLRatio:   nil,
				SourceCount: sourceCountCap + 5,
				LastSeen:    time.Now().Add(-time.Hour * hoursPerDay * recencyDecayDays * 2),
			},
			wantMin: qualityWeightSources,
			wantMax: qualityWeightSources,
		},
		{
			name: "recency half decay at 15 days yields 9-11",
			agg: DomainAggregate{
				OKRatio:     nil,
				HTMLRatio:   nil,
				SourceCount: 0,
				LastSeen:    time.Now().Add(-time.Hour * hoursPerDay * halfDecayDays),
			},
			wantMin: 9,
			wantMax: 11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agg := tt.agg
			agg.ComputeQualityScore()

			if agg.QualityScore < tt.wantMin || agg.QualityScore > tt.wantMax {
				t.Errorf(
					"QualityScore = %d, want [%d, %d]",
					agg.QualityScore, tt.wantMin, tt.wantMax,
				)
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{
			name:   "strips www prefix",
			rawURL: "https://www.example.com/page",
			want:   "example.com",
		},
		{
			name:   "no www prefix preserved",
			rawURL: "https://example.com/page",
			want:   "example.com",
		},
		{
			name:   "subdomain preserved",
			rawURL: "https://news.example.com/article",
			want:   "news.example.com",
		},
		{
			name:   "empty string returns empty",
			rawURL: "",
			want:   "",
		},
		{
			name:   "invalid URL returns empty",
			rawURL: "://broken",
			want:   "",
		},
		{
			name:   "no scheme returns empty",
			rawURL: "example.com",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ExtractDomain(tt.rawURL)
			if got != tt.want {
				t.Errorf("ExtractDomain(%q) = %q, want %q", tt.rawURL, got, tt.want)
			}
		})
	}
}
