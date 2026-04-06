package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Renderer renders JS-heavy pages and returns the HTML.
type Renderer interface {
	Render(ctx context.Context, url string) (string, error)
}

const defaultWorkBCAPIURL = "https://api-jobboard.workbc.ca/api/Search/JobSearch"

// WorkBCBoard fetches job postings from the WorkBC public JSON API.
type WorkBCBoard struct {
	apiURL     string
	httpClient *http.Client
}

// NewWorkBC creates a WorkBC board that calls the public job search API.
// The renderer parameter is accepted for interface compatibility but ignored.
func NewWorkBC(baseURL string, _ Renderer) *WorkBCBoard {
	if baseURL == "" {
		baseURL = defaultWorkBCAPIURL
	}
	return &WorkBCBoard{
		apiURL:     baseURL,
		httpClient: &http.Client{Timeout: defaultFetchTimeout},
	}
}

// Name returns the board identifier.
func (b *WorkBCBoard) Name() string { return "workbc" }

// workBCSearchRequest is the JSON body for the job search API.
type workBCSearchRequest struct {
	Page                      int    `json:"Page"`
	PageSize                  int    `json:"PageSize"`
	Keyword                   string `json:"Keyword"`
	SearchInField             string `json:"SearchInField"`
	SortOrder                 int    `json:"SortOrder"`
	SearchIsPostingsInEnglish bool   `json:"SearchIsPostingsInEnglish"`
	SearchDateSelection       int    `json:"SearchDateSelection"`
	SalaryType                int    `json:"SalaryType"`
	SearchLocationDistance     int    `json:"SearchLocationDistance"`
	SearchJobSource           string `json:"SearchJobSource"`
}

// workBCJob represents a single job from the API response.
type workBCJob struct {
	JobID          string `json:"JobId"`
	Title          string `json:"Title"`
	EmployerName   string `json:"EmployerName"`
	SalarySummary  string `json:"SalarySummary"`
	ExternalSource struct {
		Source []struct {
			URL    string `json:"Url"`
			Source string `json:"Source"`
		} `json:"Source"`
	} `json:"ExternalSource"`
}

// workBCResponse wraps the API response.
type workBCResponse struct {
	Result []workBCJob `json:"result"`
	Count  int         `json:"count"`
}

// Fetch calls the WorkBC job search API and returns postings.
func (b *WorkBCBoard) Fetch(ctx context.Context) ([]Posting, error) {
	searchReq := workBCSearchRequest{
		Page:                      1,
		PageSize:                  50,
		Keyword:                   "",
		SearchInField:             "all",
		SortOrder:                 11, // most recent
		SearchIsPostingsInEnglish: true,
		SearchDateSelection:       0,
		SalaryType:                4,
		SearchLocationDistance:     -1,
		SearchJobSource:           "0",
	}

	body, err := json.Marshal(searchReq)
	if err != nil {
		return nil, fmt.Errorf("workbc: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("workbc: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("workbc: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("workbc: HTTP %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, defaultMaxBody))
	if err != nil {
		return nil, fmt.Errorf("workbc: read body: %w", err)
	}

	var apiResp workBCResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("workbc: unmarshal response: %w", err)
	}

	var postings []Posting
	for _, job := range apiResp.Result {
		jobURL := ""
		if len(job.ExternalSource.Source) > 0 {
			jobURL = job.ExternalSource.Source[0].URL
		}

		postings = append(postings, Posting{
			Title:   job.Title,
			Company: job.EmployerName,
			URL:     jobURL,
			ID:      job.JobID,
			Body:    job.SalarySummary,
			Sector:  "government",
		})
	}

	return postings, nil
}
