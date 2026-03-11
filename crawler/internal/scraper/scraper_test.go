package scraper_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/scraper"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// newTestLogger creates a no-op logger for tests.
func newTestLogger(t *testing.T) infralogger.Logger {
	t.Helper()
	return infralogger.NewNop()
}

// communityWebsite is a test HTML page with leadership and contact links.
const communityWebsite = `<!DOCTYPE html>
<html><body>
<a href="/chief-and-council">Chief and Council</a>
<a href="/contact-us">Contact Us</a>
</body></html>`

// leadershipPage is a test leadership page with chief and councillors.
const leadershipPage = `<!DOCTYPE html>
<html><body>
<h1>Chief and Council</h1>
<p>Chief John Smith</p>
<p>Councillors</p>
<p>Jane Doe</p>
<p>Bob Wilson</p>
</body></html>`

// contactPage is a test contact page with phone, fax, and email.
const contactPage = `<!DOCTYPE html>
<html><body>
<h1>Contact Us</h1>
<p>Phone: (705) 555-1234</p>
<p>Fax: (705) 555-5678</p>
<p>Email: info@example.com</p>
<p>Postal Code: P0L 1A0</p>
</body></html>`

// apiCounters tracks the number of mutating API calls made during a test.
type apiCounters struct {
	peopleCreated  atomic.Int32
	officeUpserted atomic.Int32
	scrapedUpdated atomic.Int32
}

// testServerCfg holds configuration for the mock test server.
type testServerCfg struct {
	communities    []scraper.Community
	existingPeople []scraper.Person
	existingOffice *scraper.BandOffice // nil = 404, non-nil = returned as JSON
	counters       *apiCounters
}

// newTestServer creates a mock server that handles both source-manager API and
// community website requests.
func newTestServer(t *testing.T, cfg testServerCfg) *httptest.Server {
	t.Helper()

	var srv *httptest.Server

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleTestRequest(t, w, r, srv, &cfg)
	}))

	return srv
}

func handleTestRequest(
	t *testing.T,
	w http.ResponseWriter,
	r *http.Request,
	srv *httptest.Server,
	cfg *testServerCfg,
) {
	t.Helper()

	switch {
	case r.URL.Path == "/api/v1/communities/with-source":
		serveCommunitiesWithServerURL(t, w, srv.URL, cfg.communities)
	case strings.HasSuffix(r.URL.Path, "/people") && r.Method == http.MethodGet:
		serveJSON(t, w, map[string]any{
			"people": cfg.existingPeople,
			"total":  len(cfg.existingPeople),
		})
	case strings.HasSuffix(r.URL.Path, "/people") && r.Method == http.MethodPost:
		cfg.counters.peopleCreated.Add(1)
		w.WriteHeader(http.StatusCreated)
	case strings.HasSuffix(r.URL.Path, "/band-office") && r.Method == http.MethodGet:
		serveBandOffice(t, w, cfg.existingOffice)
	case strings.HasSuffix(r.URL.Path, "/band-office") && r.Method == http.MethodPost:
		cfg.counters.officeUpserted.Add(1)
		w.WriteHeader(http.StatusOK)
	case strings.HasSuffix(r.URL.Path, "/scraped"):
		cfg.counters.scrapedUpdated.Add(1)
		w.WriteHeader(http.StatusOK)
	case r.URL.Path == "/":
		writeHTML(w, communityWebsite)
	case r.URL.Path == "/chief-and-council":
		writeHTML(w, leadershipPage)
	case r.URL.Path == "/contact-us":
		writeHTML(w, contactPage)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// serveCommunitiesWithServerURL injects the test server URL as website for communities.
func serveCommunitiesWithServerURL(
	t *testing.T, w http.ResponseWriter, serverURL string, communities []scraper.Community,
) {
	t.Helper()
	srvURL := serverURL + "/"
	result := make([]scraper.Community, len(communities))
	copy(result, communities)
	for i := range result {
		if result[i].Website != nil {
			result[i].Website = &srvURL
		}
	}
	serveJSON(t, w, map[string]any{
		"communities": result,
		"count":       len(result),
	})
}

func serveBandOffice(t *testing.T, w http.ResponseWriter, office *scraper.BandOffice) {
	t.Helper()
	if office != nil {
		serveJSON(t, w, map[string]any{"band_office": office})
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func serveJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if encErr := json.NewEncoder(w).Encode(v); encErr != nil {
		t.Fatalf("failed to encode response: %v", encErr)
	}
}

func writeHTML(w http.ResponseWriter, html string) {
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(html))
}

func strPtr(s string) *string {
	return &s
}

func TestScraper_ScrapeCommunity(t *testing.T) {
	website := "placeholder" // will be overridden by serveCommunitiesWithServerURL
	counters := &apiCounters{}
	srv := newTestServer(t, testServerCfg{
		communities: []scraper.Community{
			{ID: "comm-1", Name: "Test Nation", Website: &website},
		},
		existingPeople: []scraper.Person{},
		counters:       counters,
	})
	defer srv.Close()

	s := scraper.New(scraper.Config{
		SourceManagerURL: srv.URL,
		Workers:          1,
	}, newTestLogger(t))

	results, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.CommunityID != "comm-1" {
		t.Errorf("expected community_id comm-1, got %s", r.CommunityID)
	}
	if r.Error != "" {
		t.Errorf("unexpected error: %s", r.Error)
	}
	if r.PeopleAdded == 0 {
		t.Error("expected people to be added")
	}
	if counters.peopleCreated.Load() == 0 {
		t.Error("expected POST /people calls")
	}
}

func TestScraper_SkipsUnchangedPeople(t *testing.T) {
	website := "placeholder"
	counters := &apiCounters{}
	srv := newTestServer(t, testServerCfg{
		communities: []scraper.Community{
			{ID: "comm-1", Name: "Test Nation", Website: &website},
		},
		existingPeople: []scraper.Person{
			{Name: "John Smith", Role: "chief"},
		},
		counters: counters,
	})
	defer srv.Close()

	s := scraper.New(scraper.Config{
		SourceManagerURL: srv.URL,
		Workers:          1,
	}, newTestLogger(t))

	results, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.PeopleSkipped == 0 {
		t.Error("expected at least one person to be skipped")
	}
}

func TestScraper_DryRun(t *testing.T) {
	website := "placeholder"
	counters := &apiCounters{}
	srv := newTestServer(t, testServerCfg{
		communities: []scraper.Community{
			{ID: "comm-1", Name: "Test Nation", Website: &website},
		},
		existingPeople: []scraper.Person{},
		counters:       counters,
	})
	defer srv.Close()

	s := scraper.New(scraper.Config{
		SourceManagerURL: srv.URL,
		Workers:          1,
		DryRun:           true,
	}, newTestLogger(t))

	results, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if counters.peopleCreated.Load() != 0 {
		t.Errorf("expected no POST /people calls in dry-run, got %d",
			counters.peopleCreated.Load())
	}
	if counters.officeUpserted.Load() != 0 {
		t.Errorf("expected no POST /band-office calls in dry-run, got %d",
			counters.officeUpserted.Load())
	}
	if counters.scrapedUpdated.Load() != 0 {
		t.Errorf("expected no PATCH /scraped calls in dry-run, got %d",
			counters.scrapedUpdated.Load())
	}

	r := results[0]
	if r.PeopleAdded == 0 && !r.OfficeUpdated {
		t.Error("expected dry-run results to be populated")
	}
}

func TestScraper_NoCommunityWebsite(t *testing.T) {
	counters := &apiCounters{}
	srv := newTestServer(t, testServerCfg{
		communities: []scraper.Community{
			{ID: "comm-1", Name: "No Website Nation", Website: nil},
		},
		existingPeople: []scraper.Person{},
		counters:       counters,
	})
	defer srv.Close()

	s := scraper.New(scraper.Config{
		SourceManagerURL: srv.URL,
		Workers:          1,
	}, newTestLogger(t))

	results, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.Error != "no website URL" {
		t.Errorf("expected 'no website URL' error, got: %s", r.Error)
	}
}

func TestScraper_SkipsUnchangedOffice(t *testing.T) {
	website := "placeholder"
	counters := &apiCounters{}

	// Return a band office whose values match what contactPage will extract.
	// contactPage contains: Phone (705) 555-1234, Fax (705) 555-5678,
	// Email info@example.com, PostalCode P0L 1A0
	existingOffice := &scraper.BandOffice{
		Phone:      strPtr("(705) 555-1234"),
		Fax:        strPtr("(705) 555-5678"),
		Email:      strPtr("info@example.com"),
		PostalCode: strPtr("P0L 1A0"),
	}

	srv := newTestServer(t, testServerCfg{
		communities: []scraper.Community{
			{ID: "comm-1", Name: "Test Nation", Website: &website},
		},
		existingPeople: []scraper.Person{},
		existingOffice: existingOffice,
		counters:       counters,
	})
	defer srv.Close()

	s := scraper.New(scraper.Config{
		SourceManagerURL: srv.URL,
		Workers:          1,
	}, newTestLogger(t))

	results, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Existing office matches extracted contact — no upsert should happen.
	if counters.officeUpserted.Load() != 0 {
		t.Errorf("expected no office upsert when unchanged, got %d",
			counters.officeUpserted.Load())
	}

	r := results[0]
	if !r.OfficeSkipped {
		t.Error("expected OfficeSkipped=true for unchanged office")
	}
}

func TestPersonKey(t *testing.T) {
	tests := []struct {
		name     string
		inName   string
		inRole   string
		expected string
	}{
		{
			name:     "basic normalization",
			inName:   "John Smith",
			inRole:   "Chief",
			expected: "john smith|chief",
		},
		{
			name:     "trims whitespace",
			inName:   "  Jane Doe  ",
			inRole:   "  councillor  ",
			expected: "jane doe|councillor",
		},
		{
			name:     "already lowercase",
			inName:   "bob wilson",
			inRole:   "councillor",
			expected: "bob wilson|councillor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scraper.PersonKeyForTest(tt.inName, tt.inRole)
			if got != tt.expected {
				t.Errorf("personKey(%q, %q) = %q, want %q",
					tt.inName, tt.inRole, got, tt.expected)
			}
		})
	}
}

func TestBandOfficeUnchanged(t *testing.T) {
	phone := "(705) 555-1234"
	email := "info@example.com"

	tests := []struct {
		name     string
		existing *scraper.BandOffice
		phone    string
		email    string
		want     bool
	}{
		{
			name: "identical values",
			existing: &scraper.BandOffice{
				Phone: &phone,
				Email: &email,
			},
			phone: phone,
			email: email,
			want:  true,
		},
		{
			name: "different phone",
			existing: &scraper.BandOffice{
				Phone: &phone,
				Email: &email,
			},
			phone: "(705) 555-9999",
			email: email,
			want:  false,
		},
		{
			name:     "nil pointer with empty string",
			existing: &scraper.BandOffice{},
			want:     true,
		},
		{
			name:     "nil pointer with non-empty value",
			existing: &scraper.BandOffice{},
			phone:    "(705) 555-1234",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scraper.BandOfficeUnchangedForTest(
				tt.existing, tt.phone, tt.email, "", "", "",
			)
			if got != tt.want {
				t.Errorf("bandOfficeUnchanged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPtrEquals(t *testing.T) {
	val := "hello"

	tests := []struct {
		name string
		ptr  *string
		val  string
		want bool
	}{
		{
			name: "nil pointer empty string",
			ptr:  nil,
			val:  "",
			want: true,
		},
		{
			name: "nil pointer non-empty string",
			ptr:  nil,
			val:  "hello",
			want: false,
		},
		{
			name: "matching values",
			ptr:  &val,
			val:  "hello",
			want: true,
		},
		{
			name: "non-matching values",
			ptr:  &val,
			val:  "world",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scraper.PtrEqualsForTest(tt.ptr, tt.val)
			if got != tt.want {
				t.Errorf("ptrEquals() = %v, want %v", got, tt.want)
			}
		})
	}
}
