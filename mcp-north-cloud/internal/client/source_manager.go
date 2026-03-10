package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SourceManagerClient is a client for the source-manager API
type SourceManagerClient struct {
	baseURL    string
	httpClient *AuthenticatedClient
}

// NewSourceManagerClient creates a new source-manager client
func NewSourceManagerClient(baseURL string, authClient *AuthenticatedClient) *SourceManagerClient {
	return &SourceManagerClient{
		baseURL:    baseURL,
		httpClient: authClient,
	}
}

// Source represents a content source
type Source struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	URL       string         `json:"url"`
	Type      string         `json:"type"`
	Selectors map[string]any `json:"selectors"`
	Enabled   bool           `json:"enabled"`
	FeedURL   *string        `json:"feed_url,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// CreateSourceRequest represents a request to create a source
type CreateSourceRequest struct {
	Name      string         `json:"name"`
	URL       string         `json:"url"`
	Type      string         `json:"type"`
	Selectors map[string]any `json:"selectors"`
	Enabled   bool           `json:"enabled"`
	FeedURL   *string        `json:"feed_url,omitempty"`
}

// UpdateSourceRequest represents a request to update a source
type UpdateSourceRequest struct {
	Name      string         `json:"name,omitempty"`
	URL       string         `json:"url,omitempty"`
	Type      string         `json:"type,omitempty"`
	Selectors map[string]any `json:"selectors,omitempty"`
	Enabled   *bool          `json:"enabled,omitempty"`
	FeedURL   *string        `json:"feed_url,omitempty"`
}

// TestCrawlRequest represents a request to test crawl a source
type TestCrawlRequest struct {
	URL       string         `json:"url"`
	Selectors map[string]any `json:"selectors"`
}

// TestCrawlResponse represents the response from test crawl
type TestCrawlResponse struct {
	Success      bool             `json:"success"`
	ArticleCount int              `json:"article_count"`
	SuccessRate  float64          `json:"success_rate"`
	Warnings     []string         `json:"warnings"`
	Articles     []map[string]any `json:"articles"`
}

// CreateSource creates a new source
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *SourceManagerClient) CreateSource(ctx context.Context, req CreateSourceRequest) (*Source, error) {
	endpoint := fmt.Sprintf("%s/api/v1/sources", c.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(respBody, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("source-manager error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var source Source
	if err = json.Unmarshal(respBody, &source); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &source, nil
}

// listSourcesPageSize is the page size used when paginating through all sources.
const listSourcesPageSize = 500

// ListSources lists all sources by paginating through the source-manager API.
func (c *SourceManagerClient) ListSources(ctx context.Context) ([]Source, error) {
	var allSources []Source
	offset := 0

	for {
		sources, total, err := c.listSourcesPage(ctx, listSourcesPageSize, offset)
		if err != nil {
			return nil, err
		}

		allSources = append(allSources, sources...)

		if len(allSources) >= total || len(sources) == 0 {
			break
		}

		offset += len(sources)
	}

	return allSources, nil
}

// listSourcesPage fetches a single page of sources from the source-manager API.
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *SourceManagerClient) listSourcesPage(ctx context.Context, limit, offset int) ([]Source, int, error) {
	endpoint := fmt.Sprintf("%s/api/v1/sources?limit=%d&offset=%d", c.baseURL, limit, offset)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Sources []Source `json:"sources"`
		Total   int      `json:"total"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, 0, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Sources, response.Total, nil
}

// GetSource gets a source by ID
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *SourceManagerClient) GetSource(ctx context.Context, sourceID string) (*Source, error) {
	endpoint := fmt.Sprintf("%s/api/v1/sources/%s", c.baseURL, sourceID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("source-manager error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var source Source
	if err = json.Unmarshal(body, &source); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &source, nil
}

// UpdateSource updates a source
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *SourceManagerClient) UpdateSource(ctx context.Context, sourceID string, req UpdateSourceRequest) (*Source, error) {
	endpoint := fmt.Sprintf("%s/api/v1/sources/%s", c.baseURL, sourceID)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(respBody, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("source-manager error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var source Source
	if err = json.Unmarshal(respBody, &source); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &source, nil
}

// DeleteSource deletes a source
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *SourceManagerClient) DeleteSource(ctx context.Context, sourceID string) error {
	endpoint := fmt.Sprintf("%s/api/v1/sources/%s", c.baseURL, sourceID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return fmt.Errorf("source-manager error: %s", errorResp.Error)
		}
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Community represents a First Nations community
type Community struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Slug             string  `json:"slug"`
	Province         string  `json:"province,omitempty"`
	Region           string  `json:"region,omitempty"`
	Nation           string  `json:"nation,omitempty"`
	TribalCouncil    string  `json:"tribal_council,omitempty"`
	Latitude         float64 `json:"latitude,omitempty"`
	Longitude        float64 `json:"longitude,omitempty"`
	Population       int     `json:"population,omitempty"`
	RegisteredPop    int     `json:"registered_pop,omitempty"`
	OnReservePop     int     `json:"on_reserve_pop,omitempty"`
	LandAreaHectares float64 `json:"land_area_hectares,omitempty"`
	Website          string  `json:"website,omitempty"`
	BandNumber       int     `json:"band_number,omitempty"`
}

// CommunityWithDistance is a community with a distance field for nearby queries.
type CommunityWithDistance struct {
	Community
	DistanceKm float64 `json:"distance_km"`
}

// ListCommunities lists all communities with optional pagination.
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *SourceManagerClient) ListCommunities(ctx context.Context, limit, offset int) ([]Community, int, error) {
	endpoint := fmt.Sprintf("%s/api/v1/communities?limit=%d&offset=%d", c.baseURL, limit, offset)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Communities []Community `json:"communities"`
		Total       int         `json:"total"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, 0, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Communities, response.Total, nil
}

// GetCommunity gets a community by ID.
//
//nolint:dupl // Similar HTTP client pattern across different services and entities is acceptable
func (c *SourceManagerClient) GetCommunity(ctx context.Context, communityID string) (*Community, error) {
	endpoint := fmt.Sprintf("%s/api/v1/communities/%s", c.baseURL, communityID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var community Community
	if err = json.Unmarshal(body, &community); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &community, nil
}

// GetCommunityBySlug gets a community by slug.
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *SourceManagerClient) GetCommunityBySlug(ctx context.Context, slug string) (*Community, error) {
	endpoint := fmt.Sprintf("%s/api/v1/communities/by-slug/%s", c.baseURL, slug)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var community Community
	if err = json.Unmarshal(body, &community); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &community, nil
}

// FindNearbyCommunities finds communities near a lat/lng point.
func (c *SourceManagerClient) FindNearbyCommunities(
	ctx context.Context, lat, lng, radiusKm float64, limit int,
) ([]CommunityWithDistance, error) {
	endpoint := fmt.Sprintf(
		"%s/api/v1/communities/nearby?lat=%f&lng=%f&radius_km=%f&limit=%d",
		c.baseURL, lat, lng, radiusKm, limit,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Communities []CommunityWithDistance `json:"communities"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Communities, nil
}

// CreateCommunity creates a new community.
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *SourceManagerClient) CreateCommunity(ctx context.Context, community Community) (*Community, error) {
	endpoint := fmt.Sprintf("%s/api/v1/communities", c.baseURL)

	reqBody, err := json.Marshal(community)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("source-manager error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var created Community
	if err = json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &created, nil
}

// UpdateCommunity updates an existing community.
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *SourceManagerClient) UpdateCommunity(ctx context.Context, communityID string, community Community) (*Community, error) {
	endpoint := fmt.Sprintf("%s/api/v1/communities/%s", c.baseURL, communityID)

	reqBody, err := json.Marshal(community)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("source-manager error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var updated Community
	if err = json.Unmarshal(body, &updated); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &updated, nil
}

// LinkSourcesResponse represents the result of source-community linking.
type LinkSourcesResponse struct {
	DryRun  bool  `json:"dry_run"`
	Matches []any `json:"matches"`
	Count   int   `json:"count"`
	Linked  int   `json:"linked,omitempty"`
}

// LinkSources matches sources to communities by name similarity.
//
//nolint:dupl // Similar HTTP client pattern across different entities is acceptable
func (c *SourceManagerClient) LinkSources(ctx context.Context, dryRun bool) (*LinkSourcesResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v1/communities/link-sources?dry_run=%t", c.baseURL, dryRun)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to link sources: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result LinkSourcesResponse
	if err = json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// Person represents a community leader or official.
type Person struct {
	ID          string  `json:"id"`
	CommunityID string  `json:"community_id"`
	Name        string  `json:"name"`
	Role        string  `json:"role"`
	IsCurrent   bool    `json:"is_current"`
	RoleTitle   *string `json:"role_title,omitempty"`
	Email       *string `json:"email,omitempty"`
	Phone       *string `json:"phone,omitempty"`
}

// BandOffice represents the physical office for a community.
type BandOffice struct {
	ID           string  `json:"id"`
	CommunityID  string  `json:"community_id"`
	AddressLine1 *string `json:"address_line1,omitempty"`
	City         *string `json:"city,omitempty"`
	Province     *string `json:"province,omitempty"`
	PostalCode   *string `json:"postal_code,omitempty"`
	Phone        *string `json:"phone,omitempty"`
	Email        *string `json:"email,omitempty"`
	TollFree     *string `json:"toll_free,omitempty"`
	OfficeHours  *string `json:"office_hours,omitempty"`
}

// ListPeople lists people for a community.
func (c *SourceManagerClient) ListPeople(ctx context.Context, communityID string, currentOnly bool, limit, offset int) ([]Person, int, error) {
	endpoint := fmt.Sprintf(
		"%s/api/v1/communities/%s/people?current_only=%t&limit=%d&offset=%d",
		c.baseURL, communityID, currentOnly, limit, offset,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var response struct {
		People []Person `json:"people"`
		Total  int      `json:"total"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, 0, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.People, response.Total, nil
}

// GetPerson gets a person by ID.
//
//nolint:dupl // Similar HTTP client pattern across different entities is acceptable
func (c *SourceManagerClient) GetPerson(ctx context.Context, personID string) (*Person, error) {
	endpoint := fmt.Sprintf("%s/api/v1/people/%s", c.baseURL, personID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var person Person
	if err = json.Unmarshal(body, &person); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &person, nil
}

// CreatePerson creates a new person in a community.
func (c *SourceManagerClient) CreatePerson(ctx context.Context, communityID string, person Person) (*Person, error) {
	endpoint := fmt.Sprintf("%s/api/v1/communities/%s/people", c.baseURL, communityID)

	reqBody, err := json.Marshal(person)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("source-manager error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var created Person
	if err = json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &created, nil
}

// GetBandOffice gets the band office for a community.
func (c *SourceManagerClient) GetBandOffice(ctx context.Context, communityID string) (*BandOffice, error) {
	endpoint := fmt.Sprintf("%s/api/v1/communities/%s/band-office", c.baseURL, communityID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var response struct {
		BandOffice BandOffice `json:"band_office"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response.BandOffice, nil
}

// UpsertBandOffice creates or updates the band office for a community.
func (c *SourceManagerClient) UpsertBandOffice(ctx context.Context, communityID string, office BandOffice) (*BandOffice, error) {
	endpoint := fmt.Sprintf("%s/api/v1/communities/%s/band-office", c.baseURL, communityID)

	reqBody, err := json.Marshal(office)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("source-manager error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var response struct {
		BandOffice BandOffice `json:"band_office"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response.BandOffice, nil
}

// TestCrawl tests crawling a source without saving
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *SourceManagerClient) TestCrawl(ctx context.Context, req TestCrawlRequest) (*TestCrawlResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v1/sources/test-crawl", c.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(respBody, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("source-manager error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var testResp TestCrawlResponse
	if err = json.Unmarshal(respBody, &testResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &testResp, nil
}
