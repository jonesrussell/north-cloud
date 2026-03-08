//nolint:testpackage // Testing unexported functions and internal handler wiring
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

func TestIsValidDomainStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"active is valid", domain.DomainStatusActive, true},
		{"ignored is valid", domain.DomainStatusIgnored, true},
		{"reviewing is valid", domain.DomainStatusReviewing, true},
		{"promoted is valid", domain.DomainStatusPromoted, true},
		{"invalid string rejected", "invalid", false},
		{"empty string rejected", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := isValidDomainStatus(tt.status)
			if got != tt.want {
				t.Errorf("isValidDomainStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestExtractPathPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{
			name:   "multi-segment yields wildcard",
			rawURL: "https://example.com/news/article/123",
			want:   "/news/*",
		},
		{
			name:   "single segment no wildcard",
			rawURL: "https://example.com/about",
			want:   "/about",
		},
		{
			name:   "root path",
			rawURL: "https://example.com/",
			want:   "/",
		},
		{
			name:   "no trailing slash",
			rawURL: "https://example.com",
			want:   "/",
		},
		{
			name:   "invalid URL returns root",
			rawURL: "://broken",
			want:   "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := extractPathPattern(tt.rawURL)
			if got != tt.want {
				t.Errorf("extractPathPattern(%q) = %q, want %q", tt.rawURL, got, tt.want)
			}
		})
	}
}

func TestExtractPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{
			name:   "normal path",
			rawURL: "https://example.com/news/article",
			want:   "/news/article",
		},
		{
			name:   "root URL returns slash",
			rawURL: "https://example.com",
			want:   "/",
		},
		{
			name:   "root with trailing slash",
			rawURL: "https://example.com/",
			want:   "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := extractPath(tt.rawURL)
			if got != tt.want {
				t.Errorf("extractPath(%q) = %q, want %q", tt.rawURL, got, tt.want)
			}
		})
	}
}

func TestComputePathClusters(t *testing.T) {
	t.Parallel()

	t.Run("empty input yields zero clusters", func(t *testing.T) {
		t.Parallel()

		clusters := computePathClusters(nil)
		if len(clusters) != 0 {
			t.Errorf("expected 0 clusters, got %d", len(clusters))
		}
	})

	t.Run("groups by pattern and sorts descending", func(t *testing.T) {
		t.Parallel()

		links := []*domain.DiscoveredLink{
			{URL: "https://example.com/news/article1"},
			{URL: "https://example.com/news/article2"},
			{URL: "https://example.com/news/article3"},
			{URL: "https://example.com/about"},
		}

		clusters := computePathClusters(links)

		expectedClusterCount := 2
		if len(clusters) != expectedClusterCount {
			t.Fatalf("expected %d clusters, got %d", expectedClusterCount, len(clusters))
		}

		// First cluster should be the most frequent
		expectedTopPattern := "/news/*"
		expectedTopCount := 3

		if clusters[0].Pattern != expectedTopPattern {
			t.Errorf("first cluster pattern = %q, want %q", clusters[0].Pattern, expectedTopPattern)
		}

		if clusters[0].Count != expectedTopCount {
			t.Errorf("first cluster count = %d, want %d", clusters[0].Count, expectedTopCount)
		}

		// Second cluster
		expectedSecondPattern := "/about"
		expectedSecondCount := 1

		if clusters[1].Pattern != expectedSecondPattern {
			t.Errorf("second cluster pattern = %q, want %q", clusters[1].Pattern, expectedSecondPattern)
		}

		if clusters[1].Count != expectedSecondCount {
			t.Errorf("second cluster count = %d, want %d", clusters[1].Count, expectedSecondCount)
		}
	})
}

// ---------------------------------------------------------------------------
// Mock types for handler HTTP tests
// ---------------------------------------------------------------------------

// bulkOverflowCount exceeds the maxBulkDomains limit (100).
const bulkOverflowCount = 101

type mockAggregateRepo struct {
	listFn    func(ctx context.Context, f database.DomainListFilters) ([]*domain.DomainAggregate, error)
	countFn   func(ctx context.Context, f database.DomainListFilters) (int, error)
	sourcesFn func(ctx context.Context, d string) ([]string, error)
	linksFn   func(ctx context.Context, d string, limit, offset int) ([]*domain.DiscoveredLink, int, error)
}

func (m *mockAggregateRepo) ListAggregates(
	ctx context.Context, f database.DomainListFilters,
) ([]*domain.DomainAggregate, error) {
	return m.listFn(ctx, f)
}

func (m *mockAggregateRepo) CountAggregates(
	ctx context.Context, f database.DomainListFilters,
) (int, error) {
	return m.countFn(ctx, f)
}

func (m *mockAggregateRepo) GetReferringSources(ctx context.Context, d string) ([]string, error) {
	return m.sourcesFn(ctx, d)
}

func (m *mockAggregateRepo) ListLinksByDomain(
	ctx context.Context, d string, limit, offset int,
) ([]*domain.DiscoveredLink, int, error) {
	return m.linksFn(ctx, d, limit, offset)
}

type mockStateRepo struct {
	upsertFn     func(ctx context.Context, d, status string, notes *string) error
	bulkUpsertFn func(ctx context.Context, domains []string, status string, notes *string) (int, error)
	getByFn      func(ctx context.Context, d string) (*domain.DomainState, error)
}

func (m *mockStateRepo) Upsert(ctx context.Context, d, status string, notes *string) error {
	return m.upsertFn(ctx, d, status, notes)
}

func (m *mockStateRepo) BulkUpsert(
	ctx context.Context, domains []string, status string, notes *string,
) (int, error) {
	return m.bulkUpsertFn(ctx, domains, status, notes)
}

func (m *mockStateRepo) GetByDomain(ctx context.Context, d string) (*domain.DomainState, error) {
	return m.getByFn(ctx, d)
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func defaultAggregateRepo() *mockAggregateRepo {
	now := time.Now()
	okRatio := 0.9
	htmlRatio := 0.8

	return &mockAggregateRepo{
		listFn: func(_ context.Context, _ database.DomainListFilters) ([]*domain.DomainAggregate, error) {
			return []*domain.DomainAggregate{{
				Domain:      "news.com",
				Status:      domain.DomainStatusActive,
				LinkCount:   10,
				SourceCount: 2,
				AvgDepth:    1.5,
				FirstSeen:   now,
				LastSeen:    now,
				OKRatio:     &okRatio,
				HTMLRatio:   &htmlRatio,
			}}, nil
		},
		countFn: func(_ context.Context, _ database.DomainListFilters) (int, error) {
			return 1, nil
		},
		sourcesFn: func(_ context.Context, _ string) ([]string, error) {
			return []string{"Source A"}, nil
		},
		linksFn: func(_ context.Context, _ string, _, _ int) ([]*domain.DiscoveredLink, int, error) {
			return []*domain.DiscoveredLink{}, 0, nil
		},
	}
}

func defaultStateRepo() *mockStateRepo {
	return &mockStateRepo{
		upsertFn: func(_ context.Context, _, _ string, _ *string) error {
			return nil
		},
		bulkUpsertFn: func(_ context.Context, domains []string, _ string, _ *string) (int, error) {
			return len(domains), nil
		},
		getByFn: func(_ context.Context, _ string) (*domain.DomainState, error) {
			return nil, nil
		},
	}
}

func setupDomainsRouter(
	aggRepo database.DomainAggregateRepositoryInterface,
	stateRepo database.DomainStateRepositoryInterface,
) *gin.Engine {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	h := NewDiscoveredDomainsHandler(aggRepo, stateRepo, infralogger.NewNop())

	v1 := r.Group("/api/v1")
	v1.GET("/discovered-domains", h.ListDomains)
	v1.GET("/discovered-domains/:domain", h.GetDomain)
	v1.GET("/discovered-domains/:domain/links", h.ListDomainLinks)
	v1.PATCH("/discovered-domains/:domain/state", h.UpdateDomainState)
	v1.POST("/discovered-domains/bulk-state", h.BulkUpdateDomainState)

	return r
}

// ---------------------------------------------------------------------------
// Handler HTTP tests
// ---------------------------------------------------------------------------

func TestListDomains_OK(t *testing.T) {
	t.Parallel()

	router := setupDomainsRouter(defaultAggregateRepo(), defaultStateRepo())

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(
		context.Background(), http.MethodGet, "/api/v1/discovered-domains", http.NoBody,
	)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var body map[string]json.RawMessage

	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if _, ok := body["domains"]; !ok {
		t.Error("expected 'domains' key in response")
	}
}

func TestGetDomain_OK(t *testing.T) {
	t.Parallel()

	router := setupDomainsRouter(defaultAggregateRepo(), defaultStateRepo())

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(
		context.Background(), http.MethodGet, "/api/v1/discovered-domains/news.com", http.NoBody,
	)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var body map[string]json.RawMessage

	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if _, ok := body["domain"]; !ok {
		t.Error("expected 'domain' key in response")
	}
}

func TestGetDomain_NotFound(t *testing.T) {
	t.Parallel()

	aggRepo := defaultAggregateRepo()
	aggRepo.listFn = func(_ context.Context, _ database.DomainListFilters) ([]*domain.DomainAggregate, error) {
		return []*domain.DomainAggregate{}, nil
	}

	router := setupDomainsRouter(aggRepo, defaultStateRepo())

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(
		context.Background(), http.MethodGet, "/api/v1/discovered-domains/nonexistent.com", http.NoBody,
	)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestUpdateDomainState_OK(t *testing.T) {
	t.Parallel()

	router := setupDomainsRouter(defaultAggregateRepo(), defaultStateRepo())

	body := `{"status":"reviewing"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(
		context.Background(), http.MethodPatch,
		"/api/v1/discovered-domains/news.com/state",
		bytes.NewBufferString(body),
	)
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d; body: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestUpdateDomainState_InvalidStatus(t *testing.T) {
	t.Parallel()

	router := setupDomainsRouter(defaultAggregateRepo(), defaultStateRepo())

	body := `{"status":"bogus"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(
		context.Background(), http.MethodPatch,
		"/api/v1/discovered-domains/news.com/state",
		bytes.NewBufferString(body),
	)
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestBulkUpdateDomainState_OK(t *testing.T) {
	t.Parallel()

	router := setupDomainsRouter(defaultAggregateRepo(), defaultStateRepo())

	body := `{"domains":["a.com","b.com"],"status":"ignored"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(
		context.Background(), http.MethodPost,
		"/api/v1/discovered-domains/bulk-state",
		bytes.NewBufferString(body),
	)
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d; body: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestBulkUpdateDomainState_EmptyDomains(t *testing.T) {
	t.Parallel()

	router := setupDomainsRouter(defaultAggregateRepo(), defaultStateRepo())

	body := `{"domains":[],"status":"ignored"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(
		context.Background(), http.MethodPost,
		"/api/v1/discovered-domains/bulk-state",
		bytes.NewBufferString(body),
	)
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestBulkUpdateDomainState_ExceedsMax(t *testing.T) {
	t.Parallel()

	router := setupDomainsRouter(defaultAggregateRepo(), defaultStateRepo())

	domains := make([]string, 0, bulkOverflowCount)
	for i := range bulkOverflowCount {
		domains = append(domains, fmt.Sprintf("d%d.com", i))
	}

	reqBody := BulkUpdateDomainStateRequest{
		Domains: domains,
		Status:  domain.DomainStatusIgnored,
	}

	bodyBytes, marshalErr := json.Marshal(reqBody)
	if marshalErr != nil {
		t.Fatalf("failed to marshal request: %v", marshalErr)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(
		context.Background(), http.MethodPost,
		"/api/v1/discovered-domains/bulk-state",
		bytes.NewBuffer(bodyBytes),
	)
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}
