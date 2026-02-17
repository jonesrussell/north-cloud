package feed_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/feed"
)

// testETagValue is the ETag used in conditional GET tests.
const testETagValue = `"abc123"`

// testLastModifiedValue is the Last-Modified value used in conditional GET tests.
const testLastModifiedValue = "Sat, 01 Jan 2024 00:00:00 GMT"

// testResponseBody is the body returned by the test server for 200 responses.
const testResponseBody = "<rss>test body</rss>"

func TestHTTPFetcher_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", testETagValue)
		w.Header().Set("Last-Modified", testLastModifiedValue)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testResponseBody))
	}))
	defer srv.Close()

	fetcher := feed.NewHTTPFetcher(srv.Client())

	resp, err := fetcher.Fetch(context.Background(), srv.URL, nil, nil)
	requireNoError(t, err)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	assertEqual(t, testResponseBody, resp.Body)

	if resp.ETag == nil || *resp.ETag != testETagValue {
		t.Errorf("expected ETag %q, got %v", testETagValue, resp.ETag)
	}

	if resp.LastModified == nil || *resp.LastModified != testLastModifiedValue {
		t.Errorf("expected Last-Modified %q, got %v", testLastModifiedValue, resp.LastModified)
	}
}

func TestHTTPFetcher_NotModified(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()

	fetcher := feed.NewHTTPFetcher(srv.Client())
	etag := testETagValue

	resp, err := fetcher.Fetch(context.Background(), srv.URL, &etag, nil)
	requireNoError(t, err)

	if resp.StatusCode != http.StatusNotModified {
		t.Errorf("expected status %d, got %d", http.StatusNotModified, resp.StatusCode)
	}

	// Body should be empty for 304.
	assertEqual(t, "", resp.Body)
}

func TestHTTPFetcher_ConditionalHeaders(t *testing.T) {
	t.Parallel()

	var receivedETag, receivedModified string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedETag = r.Header.Get("If-None-Match")
		receivedModified = r.Header.Get("If-Modified-Since")
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()

	fetcher := feed.NewHTTPFetcher(srv.Client())
	etag := testETagValue
	modified := testLastModifiedValue

	_, err := fetcher.Fetch(context.Background(), srv.URL, &etag, &modified)
	requireNoError(t, err)

	assertEqual(t, testETagValue, receivedETag)
	assertEqual(t, testLastModifiedValue, receivedModified)
}

func TestHTTPFetcher_ServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	fetcher := feed.NewHTTPFetcher(srv.Client())

	resp, err := fetcher.Fetch(context.Background(), srv.URL, nil, nil)
	requireNoError(t, err)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}

	// Body should still be readable even on error status codes.
	assertEqual(t, "internal error", resp.Body)
}

func TestHTTPFetcher_InvalidURL(t *testing.T) {
	t.Parallel()

	fetcher := feed.NewHTTPFetcher(http.DefaultClient)

	_, err := fetcher.Fetch(context.Background(), "://invalid-url", nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestHTTPFetcher_NoConditionalHeaders(t *testing.T) {
	t.Parallel()

	var hasETag, hasModified bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasETag = r.Header.Get("If-None-Match") != ""
		hasModified = r.Header.Get("If-Modified-Since") != ""
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	fetcher := feed.NewHTTPFetcher(srv.Client())

	_, err := fetcher.Fetch(context.Background(), srv.URL, nil, nil)
	requireNoError(t, err)

	if hasETag {
		t.Error("expected no If-None-Match header when etag is nil")
	}

	if hasModified {
		t.Error("expected no If-Modified-Since header when lastModified is nil")
	}
}

func TestHTTPFetcher_NoCachingHeaders(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("no cache headers"))
	}))
	defer srv.Close()

	fetcher := feed.NewHTTPFetcher(srv.Client())

	resp, err := fetcher.Fetch(context.Background(), srv.URL, nil, nil)
	requireNoError(t, err)

	if resp.ETag != nil {
		t.Errorf("expected nil ETag, got %q", *resp.ETag)
	}

	if resp.LastModified != nil {
		t.Errorf("expected nil LastModified, got %q", *resp.LastModified)
	}
}
