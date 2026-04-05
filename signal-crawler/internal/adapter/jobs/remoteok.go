package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	remoteOKTimeout      = 30 * time.Second
	remoteOKMaxBody      = 10 * 1024 * 1024 // 10 MB
	remoteOKUserAgent    = "north-cloud-signal-crawler/1.0"
	defaultRemoteOKURL   = "https://remoteok.com/api"
)

type remoteOKJob struct {
	Slug        string `json:"slug"`
	Company     string `json:"company"`
	Position    string `json:"position"`
	URL         string `json:"url"`
	ID          string `json:"id"`
	Description string `json:"description"`
}

// RemoteOKBoard fetches job postings from the RemoteOK JSON API.
type RemoteOKBoard struct {
	apiURL     string
	httpClient *http.Client
}

// NewRemoteOK creates a RemoteOK board parser. If apiURL is empty, the production URL is used.
func NewRemoteOK(apiURL string) *RemoteOKBoard {
	if apiURL == "" {
		apiURL = defaultRemoteOKURL
	}
	return &RemoteOKBoard{
		apiURL:     apiURL,
		httpClient: &http.Client{Timeout: remoteOKTimeout},
	}
}

// Name returns the board identifier.
func (b *RemoteOKBoard) Name() string { return "remoteok" }

// Fetch retrieves job postings from RemoteOK.
// The API returns a JSON array where the first element is metadata (skipped).
func (b *RemoteOKBoard) Fetch(ctx context.Context) ([]Posting, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.apiURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("remoteok: create request: %w", err)
	}
	req.Header.Set("User-Agent", remoteOKUserAgent)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remoteok: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("remoteok: HTTP %d", resp.StatusCode)
	}

	var raw []json.RawMessage
	if decErr := json.NewDecoder(io.LimitReader(resp.Body, remoteOKMaxBody)).Decode(&raw); decErr != nil {
		return nil, fmt.Errorf("remoteok: decode response: %w", decErr)
	}

	// First element is legal/metadata, skip it.
	var postings []Posting
	for i := 1; i < len(raw); i++ {
		var j remoteOKJob
		if unmarshalErr := json.Unmarshal(raw[i], &j); unmarshalErr != nil {
			continue
		}
		postings = append(postings, Posting{
			Title:   j.Position,
			Company: j.Company,
			URL:     j.URL,
			ID:      j.ID,
			Body:    j.Description,
			Sector:  "tech",
		})
	}

	return postings, nil
}
