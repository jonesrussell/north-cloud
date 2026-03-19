package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

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
