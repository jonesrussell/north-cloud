package rss_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/adapter/rss"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

const (
	testETag         = `"abc123"`
	testLastModified = "Tue, 01 Jan 2026 00:00:00 GMT"
	testBody         = `<?xml version="1.0"?><rss version="2.0"><channel></channel></rss>`
)

func makeSource(t *testing.T, url string) domain.AlertSource {
	t.Helper()

	return domain.AlertSource{
		ID:                  "test-source",
		Name:                "Test Source",
		FeedURL:             url,
		AcquisitionStrategy: domain.AcquisitionRSS,
		PollInterval:        30 * time.Minute,
	}
}

func TestFetch200(t *testing.T) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", testETag)
		w.Header().Set("Last-Modified", testLastModified)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, testBody)
	}))
	defer srv.Close()

	c := rss.New()
	out, err := c.Fetch(context.Background(), rss.FetchInput{
		Source: makeSource(t, srv.URL),
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if out.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", out.StatusCode)
	}

	if string(out.Body) != testBody {
		t.Errorf("body mismatch: got %q", string(out.Body))
	}

	if out.ETag != testETag {
		t.Errorf("expected ETag %q, got %q", testETag, out.ETag)
	}

	if out.LastModified != testLastModified {
		t.Errorf("expected Last-Modified %q, got %q", testLastModified, out.LastModified)
	}
}

func TestFetch304(t *testing.T) {
	t.Helper()

	var receivedIfNoneMatch string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedIfNoneMatch = r.Header.Get("If-None-Match")
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()

	c := rss.New()
	_, err := c.Fetch(context.Background(), rss.FetchInput{
		Source:   makeSource(t, srv.URL),
		LastETag: testETag,
	})

	if !errors.Is(err, rss.ErrNotModified) {
		t.Fatalf("expected ErrNotModified, got %v", err)
	}

	if receivedIfNoneMatch != testETag {
		t.Errorf("expected server to receive If-None-Match %q, got %q", testETag, receivedIfNoneMatch)
	}
}

func TestFetch5xxIsTransient(t *testing.T) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable) // 503
	}))
	defer srv.Close()

	c := rss.New()
	_, err := c.Fetch(context.Background(), rss.FetchInput{
		Source: makeSource(t, srv.URL),
	})

	if !errors.Is(err, rss.ErrTransient) {
		t.Fatalf("expected ErrTransient, got %v", err)
	}
}

func TestFetch4xxIsStructural(t *testing.T) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound) // 404
	}))
	defer srv.Close()

	c := rss.New()
	_, err := c.Fetch(context.Background(), rss.FetchInput{
		Source: makeSource(t, srv.URL),
	})

	if !errors.Is(err, rss.ErrStructural) {
		t.Fatalf("expected ErrStructural, got %v", err)
	}
}

func TestFetchTimeout(t *testing.T) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// Block until the client context is cancelled.
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := rss.New(rss.WithTimeout(50 * time.Millisecond))
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := c.Fetch(ctx, rss.FetchInput{
		Source: makeSource(t, srv.URL),
	})

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		// net/http wraps the context error, so check the message as a fallback.
		if !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "timeout") {
			t.Errorf("expected deadline/timeout error, got %v", err)
		}
	}
}

func TestFetchUserAgent(t *testing.T) {
	t.Helper()

	const customUA = "my-custom-agent/2.0"

	var receivedUA string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := rss.New(rss.WithUserAgent(customUA))
	_, err := c.Fetch(context.Background(), rss.FetchInput{
		Source: makeSource(t, srv.URL),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedUA != customUA {
		t.Errorf("expected User-Agent %q, got %q", customUA, receivedUA)
	}
}

func TestFetchDefaultUserAgent(t *testing.T) {
	t.Helper()

	const expectedUA = "alert-crawler/1.0 (+https://northcloud.one)"

	var receivedUA string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := rss.New()
	_, err := c.Fetch(context.Background(), rss.FetchInput{
		Source: makeSource(t, srv.URL),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedUA != expectedUA {
		t.Errorf("expected default User-Agent %q, got %q", expectedUA, receivedUA)
	}
}

func TestFetchBodySizeCap(t *testing.T) {
	t.Helper()

	const sixMB = 6 * 1024 * 1024
	const fiveMB = 5 * 1024 * 1024

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write 6 MB of zeroes — client must cap at 5 MB.
		_, _ = w.Write(bytes.Repeat([]byte("x"), sixMB))
	}))
	defer srv.Close()

	c := rss.New()
	out, err := c.Fetch(context.Background(), rss.FetchInput{
		Source: makeSource(t, srv.URL),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(out.Body) > fiveMB {
		t.Errorf("body exceeds 5 MB cap: got %d bytes", len(out.Body))
	}

	if len(out.Body) != fiveMB {
		t.Errorf("expected exactly %d bytes, got %d", fiveMB, len(out.Body))
	}
}

func TestFetchLastModifiedHeader(t *testing.T) {
	t.Helper()

	var receivedIfModifiedSince string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedIfModifiedSince = r.Header.Get("If-Modified-Since")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := rss.New()
	_, err := c.Fetch(context.Background(), rss.FetchInput{
		Source:       makeSource(t, srv.URL),
		LastModified: testLastModified,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedIfModifiedSince != testLastModified {
		t.Errorf("expected If-Modified-Since %q, got %q", testLastModified, receivedIfModifiedSince)
	}
}
