// classifier_routing_test.go
//
//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/testhelpers"
)

func TestResolveSidecars(t *testing.T) {

	routingTable := map[string][]string{
		"article":         {"crime", "mining", "location"},
		"article:event":   {"location"},
		"article:blotter": {"crime"},
		"article:report":  {},
		"page":            {},
	}
	cfg := Config{
		Version:                "1.0.0",
		MinQualityScore:        50,
		UpdateSourceRep:        true,
		QualityConfig:          QualityConfig{},
		SourceReputationConfig: SourceReputationConfig{},
		RoutingTable:           routingTable,
	}
	clf := NewClassifier(
		&mockLogger{},
		[]domain.ClassificationRule{},
		testhelpers.NewMockSourceReputationDB(),
		cfg,
	)

	tests := []struct {
		name        string
		contentType string
		subtype     string
		want        []string
	}{
		{"article default", domain.ContentTypeArticle, "", []string{"crime", "mining", "location"}},
		{"article event", domain.ContentTypeArticle, domain.ContentSubtypeEvent, []string{"location"}},
		{"article blotter", domain.ContentTypeArticle, domain.ContentSubtypeBlotter, []string{"crime"}},
		{"article unknown subtype falls back to article", domain.ContentTypeArticle, "press_release", []string{"crime", "mining", "location"}},
		{"article report", domain.ContentTypeArticle, domain.ContentSubtypeReport, nil},
		{"page has explicit empty routing", domain.ContentTypePage, "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clf.ResolveSidecars(tt.contentType, tt.subtype)
			assertEqualStringSlices(t, got, tt.want)
		})
	}
}

func assertEqualStringSlices(t *testing.T, got, want []string) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Errorf("ResolveSidecars() nil mismatch: got nil=%v, want nil=%v; got=%v, want=%v",
			got == nil, want == nil, got, want)
		return
	}
	if want == nil {
		return
	}
	if len(got) != len(want) {
		t.Errorf("ResolveSidecars() length = %d, want %d; got %v", len(got), len(want), got)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("ResolveSidecars()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResolveSidecars_MissingKey_ReturnsNilAndLogs(t *testing.T) {

	cfg := Config{
		Version:                "1.0.0",
		MinQualityScore:        50,
		UpdateSourceRep:        true,
		QualityConfig:          QualityConfig{},
		SourceReputationConfig: SourceReputationConfig{},
		RoutingTable:           map[string][]string{"article": {"crime"}}, // no "video"
	}
	clf := NewClassifier(
		&mockLogger{},
		[]domain.ClassificationRule{},
		testhelpers.NewMockSourceReputationDB(),
		cfg,
	)

	got := clf.ResolveSidecars("video", "")
	if got != nil {
		t.Errorf("ResolveSidecars(\"video\", \"\") = %v, want nil when routing key missing", got)
	}
}
