package feed

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// DefaultHTTPFetcher implements HTTPFetcher using net/http.
type DefaultHTTPFetcher struct {
	client *http.Client
}

// NewHTTPFetcher creates an HTTPFetcher backed by the given http.Client.
func NewHTTPFetcher(client *http.Client) *DefaultHTTPFetcher {
	return &DefaultHTTPFetcher{client: client}
}

// Fetch performs an HTTP GET with optional conditional headers (ETag,
// Last-Modified). It returns the status code, body, and any caching
// headers present in the response.
func (f *DefaultHTTPFetcher) Fetch(
	ctx context.Context,
	url string,
	etag, lastModified *string,
) (*FetchResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("http fetcher new request: %w", err)
	}

	setConditionalHeaders(req, etag, lastModified)

	resp, doErr := f.client.Do(req)
	if doErr != nil {
		return nil, fmt.Errorf("http fetcher do request: %w", doErr)
	}
	defer resp.Body.Close()

	return buildFetchResponse(resp)
}

// setConditionalHeaders adds If-None-Match and If-Modified-Since headers
// when non-nil values are provided.
func setConditionalHeaders(req *http.Request, etag, lastModified *string) {
	if etag != nil {
		req.Header.Set("If-None-Match", *etag)
	}

	if lastModified != nil {
		req.Header.Set("If-Modified-Since", *lastModified)
	}
}

// buildFetchResponse reads the response body and extracts caching headers.
func buildFetchResponse(resp *http.Response) (*FetchResponse, error) {
	var body string

	if resp.StatusCode != http.StatusNotModified {
		raw, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("http fetcher read body: %w", readErr)
		}

		body = string(raw)
	}

	result := &FetchResponse{
		StatusCode: resp.StatusCode,
		Body:       body,
	}

	if v := resp.Header.Get("ETag"); v != "" {
		result.ETag = &v
	}

	if v := resp.Header.Get("Last-Modified"); v != "" {
		result.LastModified = &v
	}

	return result, nil
}
