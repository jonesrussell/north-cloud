package feed_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/feed"
)

func TestFetch_ReturnsBody(t *testing.T) {
	const (
		csvBody      = "title,ref\nTest,PW-24-001\n"
		lastModified = "Mon, 03 Mar 2026 12:00:00 GMT"
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); ua != feed.UserAgentForTest {
			t.Errorf("User-Agent: expected %q, got %q", feed.UserAgentForTest, ua)
		}

		w.Header().Set("Last-Modified", lastModified)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(csvBody))
	}))
	defer srv.Close()

	f := feed.NewFetcher()
	body, modified, err := f.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !modified {
		t.Fatal("expected modified=true on first fetch")
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if string(data) != csvBody {
		t.Errorf("body: expected %q, got %q", csvBody, string(data))
	}
}

func TestFetch_NotModified(t *testing.T) {
	const lastModified = "Mon, 03 Mar 2026 12:00:00 GMT"

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		if ims := r.Header.Get("If-Modified-Since"); ims == lastModified {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("Last-Modified", lastModified)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("csv data"))
	}))
	defer srv.Close()

	f := feed.NewFetcher()

	// First fetch: should return 200 with body.
	body, modified, err := f.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("first fetch error: %v", err)
	}
	if !modified {
		t.Fatal("expected modified=true on first fetch")
	}
	body.Close()

	// Second fetch: server returns 304 because If-Modified-Since matches.
	body, modified, err = f.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("second fetch error: %v", err)
	}
	if modified {
		t.Fatal("expected modified=false on second fetch (304)")
	}
	if body != nil {
		t.Fatal("expected nil body on 304 response")
	}

	if callCount != 2 {
		t.Errorf("expected 2 server calls, got %d", callCount)
	}
}

func TestFetch_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	f := feed.NewFetcher()
	body, modified, err := f.Fetch(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if modified {
		t.Error("expected modified=false on error")
	}
	if body != nil {
		t.Error("expected nil body on error")
	}
}
