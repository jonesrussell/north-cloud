package hn

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/scoring"
)

const defaultBaseURL = "https://hacker-news.firebaseio.com"

type item struct {
	ID    int    `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
	Text  string `json:"text"`
	URL   string `json:"url"`
	By    string `json:"by"`
	Score int    `json:"score"`
}

// Adapter fetches founder intent signals from Hacker News via the Firebase API.
type Adapter struct {
	baseURL    string
	maxItems   int
	httpClient *http.Client
}

// New creates a new HN Adapter. If baseURL is empty, the production Firebase URL is used.
func New(baseURL string, maxItems int) *Adapter {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Adapter{
		baseURL:    baseURL,
		maxItems:   maxItems,
		httpClient: &http.Client{},
	}
}

// Name returns the short identifier for this adapter.
func (a *Adapter) Name() string {
	return "hn"
}

// Scan fetches recent HN stories and returns those that match founder intent signals.
func (a *Adapter) Scan(ctx context.Context) ([]adapter.Signal, error) {
	ids, err := a.fetchNewStories(ctx)
	if err != nil {
		return nil, fmt.Errorf("hn: fetch new stories: %w", err)
	}

	if len(ids) > a.maxItems {
		ids = ids[:a.maxItems]
	}

	var signals []adapter.Signal
	for _, id := range ids {
		it, err := a.fetchItem(ctx, id)
		if err != nil {
			// Skip items that fail to fetch rather than aborting the whole scan.
			continue
		}

		combined := it.Title + " " + it.Text
		score, matched := scoring.Score(combined)
		if score == 0 {
			continue
		}

		signals = append(signals, adapter.Signal{
			Label:          it.Title,
			SourceURL:      fmt.Sprintf("https://news.ycombinator.com/item?id=%d", it.ID),
			ExternalID:     strconv.Itoa(it.ID),
			SignalStrength: score,
			Notes:          "Matched: " + matched,
		})
	}

	return signals, nil
}

func (a *Adapter) fetchNewStories(ctx context.Context) ([]int, error) {
	url := a.baseURL + "/v0/newstories.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ids []int
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func (a *Adapter) fetchItem(ctx context.Context, id int) (*item, error) {
	url := fmt.Sprintf("%s/v0/item/%d.json", a.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var it item
	if err := json.NewDecoder(resp.Body).Decode(&it); err != nil {
		return nil, err
	}
	return &it, nil
}
