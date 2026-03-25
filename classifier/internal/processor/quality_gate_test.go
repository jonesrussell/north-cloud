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
		name            string
		cfg             config.QualityGateConfig
		input           []*domain.ClassifiedContent
		wantPassedCount int
		wantLowQuality  []bool
		wantRejectedIDs int
	}{
		{
			name: "gate disabled passes all through unchanged",
			cfg:  config.QualityGateConfig{Enabled: false, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 10, ContentType: domain.ContentTypePage},
				{QualityScore: 50, ContentType: domain.ContentTypeArticle},
			},
			wantPassedCount: 2,
			wantLowQuality:  []bool{false, false},
		},
		{
			name: "high quality passes through",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 70, ContentType: domain.ContentTypeArticle},
			},
			wantPassedCount: 1,
			wantLowQuality:  []bool{false},
		},
		{
			name: "low quality article flagged but passes",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 30, ContentType: domain.ContentTypeArticle},
			},
			wantPassedCount: 1,
			wantLowQuality:  []bool{true},
		},
		{
			name: "low quality page rejected",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{RawContent: domain.RawContent{ID: "page-1"}, QualityScore: 30, ContentType: domain.ContentTypePage},
			},
			wantPassedCount: 0,
			wantRejectedIDs: 1,
		},
		{
			name: "low quality event rejected",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{RawContent: domain.RawContent{ID: "evt-1"}, QualityScore: 35, ContentType: domain.ContentTypeEvent},
			},
			wantPassedCount: 0,
			wantRejectedIDs: 1,
		},
		{
			name: "threshold boundary — exactly at threshold passes",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 40, ContentType: domain.ContentTypePage},
			},
			wantPassedCount: 1,
			wantLowQuality:  []bool{false},
		},
		{
			name: "pre-existing LowQuality flag cleared when above threshold",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 70, ContentType: domain.ContentTypeArticle, LowQuality: true},
			},
			wantPassedCount: 1,
			wantLowQuality:  []bool{false},
		},
		{
			name: "mixed batch filters correctly",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 70, ContentType: domain.ContentTypeArticle},                                         // pass
				{RawContent: domain.RawContent{ID: "p-1"}, QualityScore: 30, ContentType: domain.ContentTypePage},  // reject
				{QualityScore: 35, ContentType: domain.ContentTypeArticle},                                         // flag
				{RawContent: domain.RawContent{ID: "e-1"}, QualityScore: 20, ContentType: domain.ContentTypeEvent}, // reject
				{QualityScore: 50, ContentType: domain.ContentTypeArticle},                                         // pass
			},
			wantPassedCount: 3,
			wantLowQuality:  []bool{false, true, false},
			wantRejectedIDs: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyQualityGate(tt.cfg, tt.input, logger)

			if len(result.Passed) != tt.wantPassedCount {
				t.Errorf("applyQualityGate() passed %d items, want %d", len(result.Passed), tt.wantPassedCount)
				return
			}

			if len(result.RejectedIDs) != tt.wantRejectedIDs {
				t.Errorf("applyQualityGate() rejected %d items, want %d", len(result.RejectedIDs), tt.wantRejectedIDs)
			}

			for i, wantLQ := range tt.wantLowQuality {
				if i >= len(result.Passed) {
					break
				}
				if result.Passed[i].LowQuality != wantLQ {
					t.Errorf("result.Passed[%d].LowQuality = %v, want %v (quality_score=%d, content_type=%s)",
						i, result.Passed[i].LowQuality, wantLQ, result.Passed[i].QualityScore, result.Passed[i].ContentType)
				}
			}
		})
	}
}
