package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var errGrafanaAuthFailed = errors.New("grafana authentication failed: check GRAFANA_USERNAME and GRAFANA_PASSWORD")

// GrafanaClient provides access to the Grafana HTTP API.
type GrafanaClient struct {
	baseURL    string
	httpClient *http.Client
	authHeader string
}

// NewGrafanaClient creates a new Grafana client with optional basic auth.
func NewGrafanaClient(baseURL, username, password string, authClient *AuthenticatedClient) *GrafanaClient {
	c := &GrafanaClient{
		baseURL:    baseURL,
		httpClient: authClient.HTTPClient(),
	}
	if username != "" && password != "" {
		credentials := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		c.authHeader = "Basic " + credentials
	}
	return c
}

// Alert represents a Grafana alert.
type Alert struct {
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	Status       AlertStatus       `json:"status"`
	Fingerprint  string            `json:"fingerprint"`
	GeneratorURL string            `json:"generatorURL"`
}

// AlertStatus contains the state of an alert.
type AlertStatus struct {
	State       string   `json:"state"`
	SilencedBy  []string `json:"silencedBy"`
	InhibitedBy []string `json:"inhibitedBy"`
}

// AlertGroup represents a group of alerts in Grafana.
type AlertGroup struct {
	Labels   map[string]string `json:"labels"`
	Receiver string            `json:"receiver"`
	Alerts   []Alert           `json:"alerts"`
}

// AlertsResponse represents the response from the Grafana alerts API.
type AlertsResponse struct {
	Status string     `json:"status"`
	Data   AlertsData `json:"data"`
}

// AlertsData contains the alerts grouped by receiver.
type AlertsData struct {
	Groups []AlertGroup `json:"groups"`
}

// GetActiveAlerts retrieves all active (firing) alerts from Grafana.
func (c *GrafanaClient) GetActiveAlerts(ctx context.Context) ([]Alert, error) {
	endpoint := fmt.Sprintf("%s/api/alertmanager/grafana/api/v2/alerts", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add query param to filter for active alerts only
	q := req.URL.Query()
	q.Add("active", "true")
	req.URL.RawQuery = q.Encode()

	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errGrafanaAuthFailed
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var alerts []Alert
	if decodeErr := json.NewDecoder(resp.Body).Decode(&alerts); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	return alerts, nil
}

// GetAlertGroups retrieves all alert groups from Grafana alertmanager.
func (c *GrafanaClient) GetAlertGroups(ctx context.Context) ([]AlertGroup, error) {
	endpoint := fmt.Sprintf("%s/api/alertmanager/grafana/api/v2/alerts/groups", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errGrafanaAuthFailed
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var groups []AlertGroup
	if decodeErr := json.NewDecoder(resp.Body).Decode(&groups); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	return groups, nil
}

// setAuthHeader adds the Authorization header if configured.
func (c *GrafanaClient) setAuthHeader(req *http.Request) {
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}
}
