// classifier_routing_test.go
//
//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/testhelpers"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// recordingLogger wraps mockLogger and records all Warn calls for assertion.
type recordingLogger struct {
	mockLogger
	warns []string
}

func (r *recordingLogger) Warn(msg string, _ ...infralogger.Field) {
	r.warns = append(r.warns, msg)
}

func (r *recordingLogger) With(_ ...infralogger.Field) infralogger.Logger { return r }

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

func TestResolveSidecars_MissingKey_ReturnsNil(t *testing.T) {
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

func TestNewClassifier_UnknownSidecarInRoutingTable_Warns(t *testing.T) {
	rec := &recordingLogger{}
	cfg := Config{
		RoutingTable: map[string][]string{
			"article": {"crime", "typo_sidecar"},
		},
	}
	NewClassifier(rec, []domain.ClassificationRule{}, testhelpers.NewMockSourceReputationDB(), cfg)

	found := false
	for _, w := range rec.warns {
		if strings.Contains(w, "unknown sidecar name") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a Warn containing 'unknown sidecar name', got warns: %v", rec.warns)
	}
}

func TestNewClassifier_DisabledSidecarInRoutingTable_Warns(t *testing.T) {
	rec := &recordingLogger{}
	cfg := Config{
		CrimeClassifier: nil, // crime disabled (nil)
		RoutingTable: map[string][]string{
			"article": {"crime"},
		},
	}
	NewClassifier(rec, []domain.ClassificationRule{}, testhelpers.NewMockSourceReputationDB(), cfg)

	found := false
	for _, w := range rec.warns {
		if strings.Contains(w, "disabled sidecar") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a Warn containing 'disabled sidecar', got warns: %v", rec.warns)
	}
}

func TestNewClassifier_KnownEnabledSidecar_NoWarn(t *testing.T) {
	rec := &recordingLogger{}
	mockCrime := &CrimeClassifier{}
	cfg := Config{
		CrimeClassifier: mockCrime,
		RoutingTable: map[string][]string{
			"article": {"crime"},
		},
	}
	NewClassifier(rec, []domain.ClassificationRule{}, testhelpers.NewMockSourceReputationDB(), cfg)

	for _, w := range rec.warns {
		if strings.Contains(w, "crime") {
			t.Errorf("expected no Warn for enabled crime sidecar, got: %q", w)
		}
	}
}

func TestRunOptionalClassifiers_NilSidecarDoesNotPanic(t *testing.T) {
	cfg := Config{
		CrimeClassifier: nil, // disabled
		RoutingTable: map[string][]string{
			"article": {"crime"},
		},
	}
	clf := NewClassifier(&mockLogger{}, []domain.ClassificationRule{}, testhelpers.NewMockSourceReputationDB(), cfg)
	raw := &domain.RawContent{ID: "test-nil-guard", Title: "Test Article"}

	// Must not panic when sidecar is nil but present in routing table
	crime, _, _, _, _, _ := clf.classifyOptionalForPublishable(context.Background(), raw, domain.ContentTypeArticle, "")
	if crime != nil {
		t.Error("expected nil crime result when classifier is nil")
	}
}

func TestRunOptionalClassifiers_UnknownSidecarDoesNotPanic(t *testing.T) {
	cfg := Config{
		RoutingTable: map[string][]string{
			"article": {"future_sidecar"},
		},
	}
	clf := NewClassifier(&mockLogger{}, []domain.ClassificationRule{}, testhelpers.NewMockSourceReputationDB(), cfg)
	raw := &domain.RawContent{ID: "test-unknown", Title: "Test Article"}

	// Unknown sidecar name must not panic; all results should be nil
	crime, mining, coforge, entertainment, anishinaabe, location :=
		clf.classifyOptionalForPublishable(context.Background(), raw, domain.ContentTypeArticle, "")
	if crime != nil || mining != nil || coforge != nil || entertainment != nil || anishinaabe != nil || location != nil {
		t.Error("expected all nil results for unknown sidecar name in routing table")
	}
}
