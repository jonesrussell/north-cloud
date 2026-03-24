//nolint:testpackage // Testing internal processor requires same package access
package processor

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/config"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

func TestApplyQualityGate(t *testing.T) {
	logger := newMockLoggerWithCalls()

	tests := []struct {
		name           string
		cfg            config.QualityGateConfig
		input          []*domain.ClassifiedContent
		wantCount      int
		wantLowQuality []bool
	}{
		{
			name: "gate disabled passes all through unchanged",
			cfg:  config.QualityGateConfig{Enabled: false, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 10, ContentType: "page"},
				{QualityScore: 50, ContentType: "article"},
			},
			wantCount:      2,
			wantLowQuality: []bool{false, false},
		},
		{
			name: "high quality passes through",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 70, ContentType: "article"},
			},
			wantCount:      1,
			wantLowQuality: []bool{false},
		},
		{
			name: "low quality article flagged but passes",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 30, ContentType: "article"},
			},
			wantCount:      1,
			wantLowQuality: []bool{true},
		},
		{
			name: "low quality page rejected",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 30, ContentType: "page"},
			},
			wantCount: 0,
		},
		{
			name: "low quality event rejected",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 35, ContentType: "event"},
			},
			wantCount: 0,
		},
		{
			name: "threshold boundary — exactly at threshold passes",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 40, ContentType: "page"},
			},
			wantCount:      1,
			wantLowQuality: []bool{false},
		},
		{
			name: "mixed batch filters correctly",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 70, ContentType: "article"},
				{QualityScore: 30, ContentType: "page"},
				{QualityScore: 35, ContentType: "article"},
				{QualityScore: 20, ContentType: "event"},
				{QualityScore: 50, ContentType: "article"},
			},
			wantCount:      3,
			wantLowQuality: []bool{false, true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyQualityGate(tt.cfg, tt.input, logger)

			if len(result) != tt.wantCount {
				t.Errorf("applyQualityGate() returned %d items, want %d", len(result), tt.wantCount)
				return
			}

			for i, wantLQ := range tt.wantLowQuality {
				if i >= len(result) {
					break
				}
				if result[i].LowQuality != wantLQ {
					t.Errorf("result[%d].LowQuality = %v, want %v (quality_score=%d, content_type=%s)",
						i, result[i].LowQuality, wantLQ, result[i].QualityScore, result[i].ContentType)
				}
			}
		})
	}
}
