package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

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

// LinkSourcesResponse represents the result of source-community linking.
type LinkSourcesResponse struct {
	DryRun  bool  `json:"dry_run"`
	Matches []any `json:"matches"`
	Count   int   `json:"count"`
	Linked  int   `json:"linked,omitempty"`
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
