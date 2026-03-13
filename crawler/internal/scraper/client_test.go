package scraper_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/scraper"
)

func TestClient_ListCommunitiesWithSource(t *testing.T) {
	website := "https://example.com"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/communities/with-source" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		resp := map[string]any{
			"communities": []map[string]any{
				{"id": "c1", "name": "Community One", "website": website},
				{"id": "c2", "name": "Community Two"},
			},
			"count": 2,
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	defer server.Close()

	client := scraper.NewClient(server.URL, "test-token")
	communities, err := client.ListCommunitiesWithSource(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedCount := 2
	if len(communities) != expectedCount {
		t.Fatalf("expected %d communities, got %d", expectedCount, len(communities))
	}

	if communities[0].ID != "c1" {
		t.Errorf("expected id c1, got %s", communities[0].ID)
	}

	if communities[0].Name != "Community One" {
		t.Errorf("expected name Community One, got %s", communities[0].Name)
	}

	if communities[0].Website == nil || *communities[0].Website != website {
		t.Errorf("expected website %s, got %v", website, communities[0].Website)
	}

	if communities[1].ID != "c2" {
		t.Errorf("expected id c2, got %s", communities[1].ID)
	}

	if communities[1].Website != nil {
		t.Errorf("expected nil website, got %v", communities[1].Website)
	}
}

func TestClient_ListPeople(t *testing.T) {
	email := "chief@example.com"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/communities/c1/people" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("current_only") != "true" {
			t.Errorf("expected current_only=true query param")
		}

		resp := map[string]any{
			"people": []map[string]any{
				{
					"id": "p1", "name": "Jane Doe", "role": "Chief",
					"data_source": "website", "verified": false,
					"is_current": true, "email": email,
				},
				{
					"id": "p2", "name": "John Smith", "role": "Councillor",
					"data_source": "website", "verified": false,
					"is_current": true,
				},
			},
			"total": 2, "limit": 200, "offset": 0,
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	defer server.Close()

	client := scraper.NewClient(server.URL, "test-token")
	people, err := client.ListPeople(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedCount := 2
	if len(people) != expectedCount {
		t.Fatalf("expected %d people, got %d", expectedCount, len(people))
	}

	if people[0].Name != "Jane Doe" {
		t.Errorf("expected name Jane Doe, got %s", people[0].Name)
	}

	if people[0].Role != "Chief" {
		t.Errorf("expected role Chief, got %s", people[0].Role)
	}

	if people[0].Email == nil || *people[0].Email != email {
		t.Errorf("expected email %s, got %v", email, people[0].Email)
	}

	if people[1].Name != "John Smith" {
		t.Errorf("expected name John Smith, got %s", people[1].Name)
	}
}

func TestClient_CreatePerson(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/communities/c1/people" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		var person scraper.Person
		if decodeErr := json.NewDecoder(r.Body).Decode(&person); decodeErr != nil {
			t.Fatalf("failed to decode request body: %v", decodeErr)
		}

		if person.Name != "Jane Doe" {
			t.Errorf("expected name Jane Doe, got %s", person.Name)
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := scraper.NewClient(server.URL, "test-token")
	err := client.CreatePerson(context.Background(), "c1", scraper.Person{
		Name:       "Jane Doe",
		Role:       "Chief",
		DataSource: "website",
		IsCurrent:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_GetBandOffice(t *testing.T) {
	phone := "705-555-1234"
	email := "office@band.ca"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/communities/c1/band-office" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}

		resp := map[string]any{
			"band_office": map[string]any{
				"id": "bo1", "community_id": "c1",
				"data_source": "website", "verified": false,
				"phone": phone, "email": email,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	defer server.Close()

	client := scraper.NewClient(server.URL, "test-token")
	office, err := client.GetBandOffice(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if office == nil {
		t.Fatal("expected band office, got nil")
	}

	if office.ID != "bo1" {
		t.Errorf("expected id bo1, got %s", office.ID)
	}

	if office.Phone == nil || *office.Phone != phone {
		t.Errorf("expected phone %s, got %v", phone, office.Phone)
	}

	if office.Email == nil || *office.Email != email {
		t.Errorf("expected email %s, got %v", email, office.Email)
	}
}

func TestClient_GetBandOffice_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "not found"}`))
	}))
	defer server.Close()

	client := scraper.NewClient(server.URL, "test-token")
	office, err := client.GetBandOffice(context.Background(), "c1")
	if err != nil {
		t.Fatalf("expected no error for 404, got: %v", err)
	}

	if office != nil {
		t.Errorf("expected nil band office for 404, got %+v", office)
	}
}

func TestClient_GetBandOffice_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	client := scraper.NewClient(server.URL, "test-token")
	office, err := client.GetBandOffice(context.Background(), "c1")
	if err == nil {
		t.Fatal("expected non-nil error for 500 response, got nil")
	}

	if office != nil {
		t.Errorf("expected nil band office on error, got %+v", office)
	}
}

func TestClient_UpsertBandOffice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/communities/c1/band-office" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		var office scraper.BandOffice
		if decodeErr := json.NewDecoder(r.Body).Decode(&office); decodeErr != nil {
			t.Fatalf("failed to decode request body: %v", decodeErr)
		}

		if office.DataSource != "website" {
			t.Errorf("expected data_source website, got %s", office.DataSource)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	phone := "705-555-1234"
	client := scraper.NewClient(server.URL, "test-token")
	err := client.UpsertBandOffice(context.Background(), "c1", scraper.BandOffice{
		DataSource: "website",
		Phone:      &phone,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_UpdateScrapedAt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/communities/c1/scraped" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPatch {
			t.Errorf("unexpected method: %s", r.Method)
		}

		var payload struct {
			LastScrapedAt time.Time `json:"last_scraped_at"`
		}
		if decodeErr := json.NewDecoder(r.Body).Decode(&payload); decodeErr != nil {
			t.Fatalf("failed to decode request body: %v", decodeErr)
		}

		if payload.LastScrapedAt.IsZero() {
			t.Error("expected non-zero last_scraped_at")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := scraper.NewClient(server.URL, "test-token")
	err := client.UpdateScrapedAt(context.Background(), "c1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// writeJSON is a test helper that marshals and writes JSON to the response writer.
func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}

	_, writeErr := w.Write(data)
	if writeErr != nil {
		t.Fatalf("failed to write response: %v", writeErr)
	}
}
