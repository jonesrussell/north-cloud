package api_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/api"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

func TestResolveJobType_CrawlDefaults(t *testing.T) {
	t.Helper()

	req := api.CreateJobRequest{SourceID: "src-123", URL: "https://example.com"}
	gotType, gotErr := api.ResolveJobType(&req)

	if gotErr != "" {
		t.Fatalf("unexpected error: %s", gotErr)
	}
	if gotType != domain.JobTypeCrawl {
		t.Errorf("type = %q, want %q", gotType, domain.JobTypeCrawl)
	}
}

func TestResolveJobType_ExplicitCrawl(t *testing.T) {
	t.Helper()

	req := api.CreateJobRequest{Type: domain.JobTypeCrawl, SourceID: "src-123", URL: "https://example.com"}
	gotType, gotErr := api.ResolveJobType(&req)

	if gotErr != "" {
		t.Fatalf("unexpected error: %s", gotErr)
	}
	if gotType != domain.JobTypeCrawl {
		t.Errorf("type = %q, want %q", gotType, domain.JobTypeCrawl)
	}
}

func TestResolveJobType_CrawlMissingSourceID(t *testing.T) {
	t.Helper()

	req := api.CreateJobRequest{URL: "https://example.com"}
	_, gotErr := api.ResolveJobType(&req)

	wantErr := "source_id is required for crawl jobs"
	if gotErr != wantErr {
		t.Fatalf("error = %q, want %q", gotErr, wantErr)
	}
}

func TestResolveJobType_CrawlMissingURL(t *testing.T) {
	t.Helper()

	req := api.CreateJobRequest{SourceID: "src-123"}
	_, gotErr := api.ResolveJobType(&req)

	wantErr := "url is required for crawl jobs"
	if gotErr != wantErr {
		t.Fatalf("error = %q, want %q", gotErr, wantErr)
	}
}

func TestResolveJobType_CrawlMissingBoth(t *testing.T) {
	t.Helper()

	req := api.CreateJobRequest{}
	_, gotErr := api.ResolveJobType(&req)

	wantErr := "source_id is required for crawl jobs"
	if gotErr != wantErr {
		t.Fatalf("error = %q, want %q", gotErr, wantErr)
	}
}

func TestResolveJobType_InvalidType(t *testing.T) {
	t.Helper()

	req := api.CreateJobRequest{Type: "bogus"}
	_, gotErr := api.ResolveJobType(&req)

	wantErr := "Invalid job type: bogus. Valid types: crawl, leadership_scrape"
	if gotErr != wantErr {
		t.Fatalf("error = %q, want %q", gotErr, wantErr)
	}
}

func TestResolveJobType_LeadershipDefaults(t *testing.T) {
	t.Helper()

	req := api.CreateJobRequest{Type: domain.JobTypeLeadershipScrape}
	gotType, gotErr := api.ResolveJobType(&req)

	if gotErr != "" {
		t.Fatalf("unexpected error: %s", gotErr)
	}
	if gotType != domain.JobTypeLeadershipScrape {
		t.Errorf("type = %q, want %q", gotType, domain.JobTypeLeadershipScrape)
	}
	if req.SourceID != "leadership-scrape" {
		t.Errorf("source_id = %q, want %q", req.SourceID, "leadership-scrape")
	}
	if req.URL != "leadership-scrape" {
		t.Errorf("url = %q, want %q", req.URL, "leadership-scrape")
	}
}

func TestResolveJobType_LeadershipPreservesCustomURL(t *testing.T) {
	t.Helper()

	req := api.CreateJobRequest{Type: domain.JobTypeLeadershipScrape, SourceID: "custom-src"}
	gotType, gotErr := api.ResolveJobType(&req)

	if gotErr != "" {
		t.Fatalf("unexpected error: %s", gotErr)
	}
	if gotType != domain.JobTypeLeadershipScrape {
		t.Errorf("type = %q, want %q", gotType, domain.JobTypeLeadershipScrape)
	}
	if req.SourceID != "custom-src" {
		t.Errorf("source_id = %q, want %q", req.SourceID, "custom-src")
	}
	if req.URL != "leadership-scrape" {
		t.Errorf("url = %q, want %q", req.URL, "leadership-scrape")
	}
}

func TestResolveJobType_LeadershipPreservesBoth(t *testing.T) {
	t.Helper()

	req := api.CreateJobRequest{Type: domain.JobTypeLeadershipScrape, SourceID: "custom-src", URL: "custom-url"}
	gotType, gotErr := api.ResolveJobType(&req)

	if gotErr != "" {
		t.Fatalf("unexpected error: %s", gotErr)
	}
	if gotType != domain.JobTypeLeadershipScrape {
		t.Errorf("type = %q, want %q", gotType, domain.JobTypeLeadershipScrape)
	}
	if req.SourceID != "custom-src" {
		t.Errorf("source_id = %q, want %q", req.SourceID, "custom-src")
	}
	if req.URL != "custom-url" {
		t.Errorf("url = %q, want %q", req.URL, "custom-url")
	}
}
