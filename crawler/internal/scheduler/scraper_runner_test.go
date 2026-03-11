package scheduler_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler"
)

func TestRunLeadershipScrapeJob_MissingURL(t *testing.T) {
	t.Helper()

	err := scheduler.RunLeadershipScrapeJob(context.Background(), scheduler.ScraperConfig{}, nil)
	if err == nil {
		t.Fatal("expected error for empty source-manager URL")
	}
	if !strings.Contains(err.Error(), "source-manager URL not configured") {
		t.Errorf("unexpected error: %s", err.Error())
	}
}
