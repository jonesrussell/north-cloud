package sources_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
)

func TestSourceClient_Interface(t *testing.T) {
	t.Helper()

	// Verify interface is defined
	var _ sources.Client = (*sources.HTTPClient)(nil)
}

func TestSource_HasRequiredFields(t *testing.T) {
	t.Helper()

	s := sources.Source{
		ID:        uuid.New(),
		Name:      "Test",
		URL:       "https://example.com",
		RateLimit: 10,
		MaxDepth:  2,
		Enabled:   true,
		Priority:  "normal",
	}

	if s.ID == uuid.Nil {
		t.Error("ID should not be nil")
	}
	if s.Name == "" {
		t.Error("Name should not be empty")
	}
	if s.URL == "" {
		t.Error("URL should not be empty")
	}
	if s.RateLimit == 0 {
		t.Error("RateLimit should not be zero")
	}
	if s.MaxDepth == 0 {
		t.Error("MaxDepth should not be zero")
	}
	if !s.Enabled {
		t.Error("Enabled should be true")
	}
	if s.Priority == "" {
		t.Error("Priority should not be empty")
	}
}

func TestNoOpClient_ReturnsNotFoundError(t *testing.T) {
	t.Helper()

	client := sources.NewNoOpClient()
	source, err := client.GetSource(context.Background(), uuid.New())

	if !errors.Is(err, sources.ErrSourceNotFound) {
		t.Errorf("expected ErrSourceNotFound, got %v", err)
	}
	if source != nil {
		t.Error("expected nil source from NoOpClient")
	}
}

func TestListIndigenousSources_ReturnsSources(t *testing.T) {
	t.Helper()

	region := "canada"
	want := []*sources.SourceListItem{
		{ID: uuid.New(), Name: "APTN", URL: "https://aptn.ca", Enabled: true, IndigenousRegion: &region},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		if r.URL.Path != "/api/v1/sources/indigenous" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if !strings.Contains(r.URL.RawQuery, "limit=500") {
			t.Errorf("expected limit=500 in query, got: %s", r.URL.RawQuery)
		}
		payload := map[string]any{"sources": want, "total": 1}
		if encErr := json.NewEncoder(w).Encode(payload); encErr != nil {
			t.Errorf("encode response: %v", encErr)
		}
	}))
	defer srv.Close()

	client := sources.NewHTTPClient(srv.URL, nil)
	got, err := client.ListIndigenousSources(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d sources, got %d", len(want), len(got))
	}
	if got[0].Name != want[0].Name {
		t.Errorf("expected name %q, got %q", want[0].Name, got[0].Name)
	}
}

func TestListIndigenousSources_NullSources_ReturnsEmptySlice(t *testing.T) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Helper()
		fmt.Fprint(w, `{"sources":null,"total":0}`)
	}))
	defer srv.Close()

	client := sources.NewHTTPClient(srv.URL, nil)
	got, err := client.ListIndigenousSources(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 sources, got %d", len(got))
	}
}

func TestListIndigenousSources_NonOKStatus_ReturnsError(t *testing.T) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Helper()
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := sources.NewHTTPClient(srv.URL, nil)
	_, err := client.ListIndigenousSources(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 status, got nil")
	}
}
