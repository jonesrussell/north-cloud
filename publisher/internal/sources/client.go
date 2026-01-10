package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jonesrussell/north-cloud/publisher/internal/config"
	infrahttp "github.com/north-cloud/infrastructure/http"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

type Client struct {
	url     string
	timeout time.Duration
	logger  infralogger.Logger
}

type CitiesResponse struct {
	Cities []City `json:"cities"`
	Count  int    `json:"count"`
}

type City struct {
	Name    string `json:"name"`
	Index   string `json:"index"`
	GroupID string `json:"group_id,omitempty"`
}

func NewClient(cfg *config.SourcesConfig, log infralogger.Logger) *Client {
	return &Client{
		url:     cfg.URL,
		timeout: cfg.Timeout,
		logger:  log,
	}
}

func (c *Client) GetCities(ctx context.Context) ([]config.CityConfig, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	url := fmt.Sprintf("%s/api/v1/cities", c.url)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	client := infrahttp.NewClient(&infrahttp.ClientConfig{
		Timeout: c.timeout,
	})

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		c.logger.Warn("Failed to fetch cities from sources service",
			infralogger.String("url", url),
			infralogger.Duration("duration", duration),
			infralogger.Error(err),
		)
		return nil, fmt.Errorf("fetch cities: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("Sources service returned non-OK status",
			infralogger.String("url", url),
			infralogger.Int("status_code", resp.StatusCode),
			infralogger.Duration("duration", duration),
		)
		return nil, fmt.Errorf("sources service returned status %d", resp.StatusCode)
	}

	var citiesResp CitiesResponse
	if err = json.NewDecoder(resp.Body).Decode(&citiesResp); err != nil {
		c.logger.Error("Failed to decode cities response",
			infralogger.String("url", url),
			infralogger.Duration("duration", duration),
			infralogger.Error(err),
		)
		return nil, fmt.Errorf("decode response: %w", err)
	}

	c.logger.Info("Fetched cities from sources service",
		infralogger.String("url", url),
		infralogger.Int("city_count", citiesResp.Count),
		infralogger.Duration("duration", duration),
	)

	cities := make([]config.CityConfig, 0, len(citiesResp.Cities))
	for _, city := range citiesResp.Cities {
		cities = append(cities, config.CityConfig{
			Name:  city.Name,
			Index: city.Index,
		})
	}

	return cities, nil
}
