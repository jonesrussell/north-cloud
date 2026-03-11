package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client calls source-manager API endpoints for the leadership scraper.
type Client struct {
	baseURL    string
	httpClient *http.Client
	jwtToken   string
}

const clientTimeout = 30 * time.Second

// NewClient creates a new source-manager API client.
func NewClient(baseURL, jwtToken string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: clientTimeout},
		jwtToken:   jwtToken,
	}
}

// Community represents a community from source-manager.
type Community struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Website *string `json:"website,omitempty"`
}

// Person represents a person from source-manager.
type Person struct {
	ID          string  `json:"id,omitempty"`
	CommunityID string  `json:"community_id,omitempty"`
	Name        string  `json:"name"`
	Role        string  `json:"role"`
	DataSource  string  `json:"data_source"`
	Verified    bool    `json:"verified"`
	IsCurrent   bool    `json:"is_current"`
	Email       *string `json:"email,omitempty"`
	Phone       *string `json:"phone,omitempty"`
	SourceURL   *string `json:"source_url,omitempty"`
}

// BandOffice represents a band office from source-manager.
type BandOffice struct {
	ID          string  `json:"id,omitempty"`
	CommunityID string  `json:"community_id,omitempty"`
	DataSource  string  `json:"data_source"`
	Verified    bool    `json:"verified"`
	Phone       *string `json:"phone,omitempty"`
	Fax         *string `json:"fax,omitempty"`
	Email       *string `json:"email,omitempty"`
	TollFree    *string `json:"toll_free,omitempty"`
	PostalCode  *string `json:"postal_code,omitempty"`
	SourceURL   *string `json:"source_url,omitempty"`
}

// ListCommunitiesWithSource returns communities that have a linked source and website.
func (c *Client) ListCommunitiesWithSource(ctx context.Context) ([]Community, error) {
	url := c.baseURL + "/api/v1/communities/with-source"

	body, err := c.doGet(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("list communities with source: %w", err)
	}

	var resp struct {
		Communities []Community `json:"communities"`
	}

	if unmarshalErr := json.Unmarshal(body, &resp); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal communities response: %w", unmarshalErr)
	}

	return resp.Communities, nil
}

// ListPeople returns people for a community.
func (c *Client) ListPeople(ctx context.Context, communityID string) ([]Person, error) {
	url := fmt.Sprintf(
		"%s/api/v1/communities/%s/people?current_only=true&limit=200",
		c.baseURL, communityID,
	)

	body, err := c.doGet(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("list people: %w", err)
	}

	var resp struct {
		People []Person `json:"people"`
	}

	if unmarshalErr := json.Unmarshal(body, &resp); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal people response: %w", unmarshalErr)
	}

	return resp.People, nil
}

// CreatePerson creates a person for a community.
func (c *Client) CreatePerson(ctx context.Context, communityID string, p Person) error {
	url := fmt.Sprintf("%s/api/v1/communities/%s/people", c.baseURL, communityID)

	body, marshalErr := json.Marshal(p)
	if marshalErr != nil {
		return fmt.Errorf("marshal person: %w", marshalErr)
	}

	if err := c.doPost(ctx, url, body); err != nil {
		return fmt.Errorf("create person: %w", err)
	}

	return nil
}

// GetBandOffice returns the band office for a community, or nil if not found.
func (c *Client) GetBandOffice(ctx context.Context, communityID string) (*BandOffice, error) {
	url := fmt.Sprintf("%s/api/v1/communities/%s/band-office", c.baseURL, communityID)

	body, getErr := c.doGet(ctx, url)
	if getErr != nil {
		return nil, nil //nolint:nilerr,nilnil // 404 returns error from doGet; nil,nil = "not found"
	}

	var resp struct {
		BandOffice BandOffice `json:"band_office"`
	}

	if unmarshalErr := json.Unmarshal(body, &resp); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal band office response: %w", unmarshalErr)
	}

	return &resp.BandOffice, nil
}

// UpsertBandOffice creates or updates the band office for a community.
func (c *Client) UpsertBandOffice(ctx context.Context, communityID string, b BandOffice) error {
	url := fmt.Sprintf("%s/api/v1/communities/%s/band-office", c.baseURL, communityID)

	body, marshalErr := json.Marshal(b)
	if marshalErr != nil {
		return fmt.Errorf("marshal band office: %w", marshalErr)
	}

	if err := c.doPost(ctx, url, body); err != nil {
		return fmt.Errorf("upsert band office: %w", err)
	}

	return nil
}

// UpdateScrapedAt sets the last_scraped_at timestamp for a community.
func (c *Client) UpdateScrapedAt(ctx context.Context, communityID string, scrapedAt time.Time) error {
	url := fmt.Sprintf("%s/api/v1/communities/%s/scraped", c.baseURL, communityID)

	payload := struct {
		LastScrapedAt time.Time `json:"last_scraped_at"`
	}{LastScrapedAt: scrapedAt}

	body, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return fmt.Errorf("marshal scraped-at payload: %w", marshalErr)
	}

	if err := c.doPatch(ctx, url, body); err != nil {
		return fmt.Errorf("update scraped at: %w", err)
	}

	return nil
}

// doGet performs an authenticated GET request and returns the response body.
func (c *Client) doGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create GET request: %w", err)
	}

	return c.doRequest(req)
}

// doPost performs an authenticated POST request.
func (c *Client) doPost(ctx context.Context, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create POST request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	_, doErr := c.doRequest(req)

	return doErr
}

// doPatch performs an authenticated PATCH request.
func (c *Client) doPatch(ctx context.Context, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create PATCH request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	_, doErr := c.doRequest(req)

	return doErr
}

// doRequest executes the HTTP request with auth header and handles the response.
func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	if c.jwtToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.jwtToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("read response body: %w", readErr)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
