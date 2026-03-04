package feed

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const userAgent = "NorthCloud-RFP-Ingestor/0.1.0"

// Fetcher downloads CSV feeds over HTTP with conditional request support.
// It tracks Last-Modified headers per URL and sends If-Modified-Since on
// subsequent requests, avoiding redundant downloads when the feed has not
// changed.
type Fetcher struct {
	client       *http.Client
	lastModified map[string]string
	mu           sync.Mutex
}

// NewFetcher returns a Fetcher with a 60-second HTTP timeout.
func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		lastModified: make(map[string]string),
	}
}

// Fetch downloads the resource at url. It returns the response body, a
// boolean indicating whether the content was modified since the last fetch,
// and any error encountered.
//
// When the server responds with 304 Not Modified the returned body is nil
// and modified is false. The caller is responsible for closing the body
// when modified is true.
func (f *Fetcher) Fetch(ctx context.Context, url string) (io.ReadCloser, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)

	f.mu.Lock()
	if lm, ok := f.lastModified[url]; ok {
		req.Header.Set("If-Modified-Since", lm)
	}
	f.mu.Unlock()

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("execute request: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		if lm := resp.Header.Get("Last-Modified"); lm != "" {
			f.mu.Lock()
			f.lastModified[url] = lm
			f.mu.Unlock()
		}
		return resp.Body, true, nil

	case http.StatusNotModified:
		resp.Body.Close()
		return nil, false, nil

	default:
		resp.Body.Close()
		return nil, false, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}
}
