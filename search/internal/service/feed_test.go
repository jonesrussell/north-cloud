package service_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/search/internal/service"
)

func TestFeedFilterForSlug(t *testing.T) {
	tests := []struct {
		name       string
		slug       string
		wantTopics []string
		wantMin    int
	}{
		{
			name:       "crime returns five sub-categories",
			slug:       "crime",
			wantTopics: []string{"violent_crime", "property_crime", "drug_crime", "organized_crime", "criminal_justice"},
			wantMin:    service.ExportedTopicFeedMinQuality,
		},
		{
			name:       "mining returns mining topic",
			slug:       "mining",
			wantTopics: []string{"mining"},
			wantMin:    service.ExportedTopicFeedMinQuality,
		},
		{
			name:       "entertainment returns entertainment topic",
			slug:       "entertainment",
			wantTopics: []string{"entertainment"},
			wantMin:    service.ExportedTopicFeedMinQuality,
		},
		{
			name:       "pipeline returns nil topics with higher quality",
			slug:       "pipeline",
			wantTopics: nil,
			wantMin:    service.ExportedPipelineFeedMinQuality,
		},
		{
			name:       "unknown slug defaults to pipeline",
			slug:       "unknown",
			wantTopics: nil,
			wantMin:    service.ExportedPipelineFeedMinQuality,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTopics, gotMin := service.FeedFilterForSlug(tt.slug)
			assertMinQuality(t, tt.slug, gotMin, tt.wantMin)
			assertTopics(t, tt.slug, gotTopics, tt.wantTopics)
		})
	}
}

func assertMinQuality(t *testing.T, slug string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("feedFilterForSlug(%q) minQuality = %d, want %d", slug, got, want)
	}
}

func assertTopics(t *testing.T, slug string, got, want []string) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Errorf("feedFilterForSlug(%q) topics = %v, want nil", slug, got)
		}
		return
	}
	if len(got) != len(want) {
		t.Errorf("feedFilterForSlug(%q) topics length = %d, want %d", slug, len(got), len(want))
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("feedFilterForSlug(%q) topics[%d] = %q, want %q", slug, i, got[i], want[i])
		}
	}
}
