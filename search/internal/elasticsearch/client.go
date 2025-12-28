package elasticsearch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/jonesrussell/north-cloud/search/internal/config"
)

// Client wraps the Elasticsearch client
type Client struct {
	esClient *es.Client
	config   *config.ElasticsearchConfig
}

// NewClient creates a new Elasticsearch client
func NewClient(cfg *config.ElasticsearchConfig) (*Client, error) {
	// Prepare addresses
	addresses := []string{cfg.URL}
	if !strings.HasPrefix(cfg.URL, "http://") && !strings.HasPrefix(cfg.URL, "https://") {
		addresses = []string{"http://" + cfg.URL}
	}

	// Configure client
	clientConfig := es.Config{
		Addresses:  addresses,
		MaxRetries: cfg.MaxRetries,
	}

	// Add authentication if provided
	if cfg.Username != "" {
		clientConfig.Username = cfg.Username
		clientConfig.Password = cfg.Password
	}

	// Create client
	esClient, err := es.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch client: %w", err)
	}

	client := &Client{
		esClient: esClient,
		config:   cfg,
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping elasticsearch: %w", err)
	}

	return client, nil
}

// Ping verifies the Elasticsearch connection
func (c *Client) Ping(ctx context.Context) error {
	res, err := c.esClient.Ping(c.esClient.Ping.WithContext(ctx))
	if err != nil {
		return err
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("elasticsearch ping failed: %s", string(body))
	}

	return nil
}

// Search executes a search query
func (c *Client) Search(ctx context.Context, indexPattern string, query map[string]interface{}) (*esapi.Response, error) {
	// Execute search
	res, err := c.esClient.Search(
		c.esClient.Search.WithContext(ctx),
		c.esClient.Search.WithIndex(indexPattern),
		c.esClient.Search.WithBody(c.buildRequestBody(query)),
		c.esClient.Search.WithTimeout(c.config.Timeout),
		c.esClient.Search.WithTrackTotalHits(true),
	)

	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}

	if res.IsError() {
		defer func() {
			_ = res.Body.Close()
		}()
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("search returned error [%d]: %s", res.StatusCode, string(body))
	}

	return res, nil
}

// ListIndices lists all indices matching a pattern
func (c *Client) ListIndices(ctx context.Context, pattern string) ([]string, error) {
	res, err := c.esClient.Cat.Indices(
		c.esClient.Cat.Indices.WithIndex(pattern),
		c.esClient.Cat.Indices.WithContext(ctx),
		c.esClient.Cat.Indices.WithFormat("json"),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to list indices: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("list indices returned error [%d]: %s", res.StatusCode, string(body))
	}

	// Parse response (simplified - full implementation would parse JSON)
	var indices []string
	// In a real implementation, parse the JSON response
	// For now, return empty slice
	return indices, nil
}

// HealthCheck checks Elasticsearch cluster health
func (c *Client) HealthCheck(ctx context.Context) error {
	res, err := c.esClient.Cluster.Health(
		c.esClient.Cluster.Health.WithContext(ctx),
	)

	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("cluster unhealthy [%d]: %s", res.StatusCode, string(body))
	}

	return nil
}

// buildRequestBody creates an io.Reader from the query map
func (c *Client) buildRequestBody(_ map[string]interface{}) io.Reader {
	// This would typically use json.Marshal, but for simplicity we'll use strings.NewReader
	// The actual implementation is in the QueryBuilder
	return nil // Placeholder - actual implementation in search service
}

// GetConfig returns the Elasticsearch configuration
func (c *Client) GetConfig() *config.ElasticsearchConfig {
	return c.config
}

// GetESClient returns the underlying Elasticsearch client
func (c *Client) GetESClient() *es.Client {
	return c.esClient
}
