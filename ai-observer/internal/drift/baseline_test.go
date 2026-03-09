package drift_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/drift"
)

func TestBuildCategoryDistribution(t *testing.T) {
	t.Helper()
	docs := []drift.ClassifiedDoc{
		{Topics: []string{"tech", "sports"}, SourceRegion: "north_america"},
		{Topics: []string{"tech"}, SourceRegion: "north_america"},
		{Topics: []string{"politics"}, SourceRegion: "oceania"},
	}

	dist := drift.BuildCategoryDistribution(docs)

	if got := dist["global"]["tech"]; got < 0.49 || got > 0.51 {
		t.Errorf("expected global tech ~0.50, got %f", got)
	}
	if got := dist["north_america"]["tech"]; got < 0.66 || got > 0.67 {
		t.Errorf("expected north_america tech ~0.67, got %f", got)
	}
}

func TestBuildConfidenceHistograms(t *testing.T) {
	t.Helper()
	docs := []drift.ClassifiedDoc{
		{Confidence: 0.15, CrimeConfidence: 0.8},
		{Confidence: 0.85, CrimeConfidence: 0.2},
		{Confidence: 0.15},
	}

	histograms := drift.BuildConfidenceHistograms(docs)

	global := histograms["global"]
	if len(global) != drift.HistogramBins {
		t.Fatalf("expected %d bins, got %d", drift.HistogramBins, len(global))
	}
	if global[1] < 0.66 || global[1] > 0.67 {
		t.Errorf("expected bin[1] ~0.67, got %f", global[1])
	}
}

func TestBuildCrossMatrix(t *testing.T) {
	t.Helper()
	docs := []drift.ClassifiedDoc{
		{Topics: []string{"tech"}, SourceRegion: "north_america"},
		{Topics: []string{"tech"}, SourceRegion: "north_america"},
		{Topics: []string{"politics"}, SourceRegion: "north_america"},
		{Topics: []string{"indigenous"}, SourceRegion: "oceania"},
	}

	matrix, counts := drift.BuildCrossMatrix(docs)

	if got := matrix["north_america"]["tech"]; got < 0.66 || got > 0.67 {
		t.Errorf("expected north_america/tech ~0.67, got %f", got)
	}
	if got := counts["north_america"]["tech"]; got != 2 {
		t.Errorf("expected north_america/tech count 2, got %d", got)
	}
}
