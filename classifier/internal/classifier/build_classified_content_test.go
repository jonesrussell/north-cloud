//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestBuildClassifiedContentPreservesOptionalResults(t *testing.T) {
	c := NewClassifier(&mockLogger{}, nil, nil, Config{Version: "test"})
	raw := &domain.RawContent{
		ID:         "doc-1",
		URL:        "https://example.com/story",
		SourceName: "example",
		Title:      "Northern Ontario business story",
		RawText:    "A consulting firm in Sudbury is expanding.",
	}
	result := &domain.ClassificationResult{
		ContentType:          domain.ContentTypeArticle,
		QualityScore:         88,
		QualityFactors:       map[string]any{"word_count": 1.0},
		Topics:               []string{"business"},
		TopicScores:          map[string]float64{"business": 0.91},
		SourceReputation:     72,
		SourceCategory:       "news",
		ClassifierVersion:    "test",
		ClassificationMethod: "rule_based",
		Confidence:           0.91,
		NeedSignal: &domain.NeedSignalResult{
			SignalType:       "expansion",
			OrganizationName: "Example Consulting",
			Confidence:       0.77,
		},
		ICP: &domain.ICPResult{
			ModelVersion: "icp-segments-v1",
			Segments: []domain.ICPSegmentResult{
				{
					Segment:         "northern_ontario_industry",
					Score:           0.82,
					MatchedKeywords: []string{"sudbury"},
				},
			},
		},
	}

	classified := c.BuildClassifiedContent(raw, result)

	require.Same(t, result.NeedSignal, classified.NeedSignal)
	require.Same(t, result.ICP, classified.ICP)
	require.Equal(t, raw.RawText, classified.Body)
	require.Equal(t, raw.URL, classified.Source)
}
