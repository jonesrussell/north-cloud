package elasticsearch

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
)

const unknownStatus = "unknown"

// Client wraps the Elasticsearch client with index management operations
type Client struct {
	esClient *es.Client
	config   *Config
}

// Config holds Elasticsearch configuration
type Config struct {
	URL        string
	Username   string
	Password   string
	MaxRetries int
	Timeout    time.Duration
}

// NewClient creates a new Elasticsearch client
func NewClient(cfg *Config) (*Client, error) {
	// Parse URL to get addresses
	addresses := []string{cfg.URL}
	if !strings.HasPrefix(cfg.URL, "http://") && !strings.HasPrefix(cfg.URL, "https://") {
		addresses = []string{"http://" + cfg.URL}
	}

	// Create transport
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
	}

	// Create client config
	clientConfig := es.Config{
		Addresses:  addresses,
		Transport:  transport,
		MaxRetries: cfg.MaxRetries,
	}

	// Configure authentication
	if cfg.Username != "" && cfg.Password != "" {
		clientConfig.Username = cfg.Username
		clientConfig.Password = cfg.Password
	}

	// Create client
	esClient, err := es.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}

	// Verify connection
	res, err := esClient.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping Elasticsearch: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.IsError() {
		return nil, fmt.Errorf("error pinging Elasticsearch: %s", res.String())
	}

	return &Client{
		esClient: esClient,
		config:   cfg,
	}, nil
}

// GetClient returns the underlying Elasticsearch client
func (c *Client) GetClient() *es.Client {
	return c.esClient
}

// IndexInfo represents information about an Elasticsearch index
type IndexInfo struct {
	Name          string                 `json:"name"`
	Health        string                 `json:"health"`
	Status        string                 `json:"status"`
	DocumentCount int64                  `json:"document_count"`
	Size          string                 `json:"size"`
	Settings      map[string]interface{} `json:"settings,omitempty"`
	Mappings      map[string]interface{} `json:"mappings,omitempty"`
}

// CreateIndex creates a new index with the specified mapping
func (c *Client) CreateIndex(ctx context.Context, indexName string, mapping interface{}) error {
	// Check if index already exists
	exists, err := c.IndexExists(ctx, indexName)
	if err != nil {
		return fmt.Errorf("failed to check if index exists: %w", err)
	}
	if exists {
		return fmt.Errorf("index %s already exists", indexName)
	}

	// Convert mapping to JSON
	var mappingReader io.Reader
	if mapping != nil {
		mappingBytes, marshalErr := json.Marshal(mapping)
		if marshalErr != nil {
			return fmt.Errorf("failed to marshal mapping: %w", marshalErr)
		}
		mappingReader = strings.NewReader(string(mappingBytes))
	}

	// Create index
	res, err := c.esClient.Indices.Create(indexName, c.esClient.Indices.Create.WithBody(mappingReader), c.esClient.Indices.Create.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("error creating index: %s", string(body))
	}

	return nil
}

// EnsureIndex ensures an index exists, creating it if it doesn't
func (c *Client) EnsureIndex(ctx context.Context, indexName string, mapping interface{}) error {
	exists, err := c.IndexExists(ctx, indexName)
	if err != nil {
		return fmt.Errorf("failed to check if index exists: %w", err)
	}
	if exists {
		return nil
	}

	return c.CreateIndex(ctx, indexName, mapping)
}

// DeleteIndex deletes an index
func (c *Client) DeleteIndex(ctx context.Context, indexName string) error {
	res, err := c.esClient.Indices.Delete([]string{indexName}, c.esClient.Indices.Delete.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to delete index: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("error deleting index: %s", string(body))
	}

	return nil
}

// IndexExists checks if an index exists
func (c *Client) IndexExists(ctx context.Context, indexName string) (bool, error) {
	res, err := c.esClient.Indices.Exists([]string{indexName}, c.esClient.Indices.Exists.WithContext(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to check index existence: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if res.IsError() {
		return false, fmt.Errorf("error checking index existence: %s", res.String())
	}

	return true, nil
}

// ListIndices lists all indices matching the pattern
func (c *Client) ListIndices(ctx context.Context, pattern string) ([]string, error) {
	var indices []string
	if pattern == "" {
		pattern = "*"
	}

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
		return nil, fmt.Errorf("error listing indices: %s", string(body))
	}

	var results []map[string]interface{}
	if decodeErr := json.NewDecoder(res.Body).Decode(&results); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	for _, result := range results {
		if name, ok := result["index"].(string); ok {
			// Filter out system indices
			if !strings.HasPrefix(name, ".") {
				indices = append(indices, name)
			}
		}
	}

	return indices, nil
}

// extractDocumentCount extracts document count from stats data
func extractDocumentCount(statsData map[string]interface{}, indexName string) int64 {
	indices, ok1 := statsData["indices"].(map[string]interface{})
	if !ok1 {
		return 0
	}

	indexStats, ok2 := indices[indexName].(map[string]interface{})
	if !ok2 {
		return 0
	}

	total, ok3 := indexStats["total"].(map[string]interface{})
	if !ok3 {
		return 0
	}

	docs, ok4 := total["docs"].(map[string]interface{})
	if !ok4 {
		return 0
	}

	count, ok5 := docs["count"].(float64)
	if !ok5 {
		return 0
	}

	return int64(count)
}

// extractHealthStatus extracts health and status from health data
func extractHealthStatus(healthData map[string]interface{}, indexName string) (health, status string) {
	health = unknownStatus
	if h, ok1 := healthData["status"].(string); ok1 {
		health = h
	}

	status = unknownStatus
	if indices, ok1 := healthData["indices"].(map[string]interface{}); ok1 {
		if indexHealth, ok2 := indices[indexName].(map[string]interface{}); ok2 {
			if s, ok3 := indexHealth["status"].(string); ok3 {
				status = s
			}
		}
	}

	return health, status
}

// GetIndexInfo gets detailed information about an index
func (c *Client) GetIndexInfo(ctx context.Context, indexName string) (*IndexInfo, error) {
	// Get index stats
	statsRes, err := c.esClient.Indices.Stats(
		c.esClient.Indices.Stats.WithIndex(indexName),
		c.esClient.Indices.Stats.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get index stats: %w", err)
	}
	defer func() {
		_ = statsRes.Body.Close()
	}()

	if statsRes.IsError() {
		body, _ := io.ReadAll(statsRes.Body)
		return nil, fmt.Errorf("error getting index stats: %s", string(body))
	}

	var statsData map[string]interface{}
	if statsDecodeErr := json.NewDecoder(statsRes.Body).Decode(&statsData); statsDecodeErr != nil {
		return nil, fmt.Errorf("failed to decode stats: %w", statsDecodeErr)
	}

	// Get index health
	healthRes, healthErr := c.esClient.Cluster.Health(
		c.esClient.Cluster.Health.WithIndex(indexName),
		c.esClient.Cluster.Health.WithContext(ctx),
	)
	if healthErr != nil {
		return nil, fmt.Errorf("failed to get index health: %w", healthErr)
	}
	defer func() {
		_ = healthRes.Body.Close()
	}()

	var healthData map[string]interface{}
	if healthDecodeErr := json.NewDecoder(healthRes.Body).Decode(&healthData); healthDecodeErr != nil {
		return nil, fmt.Errorf("failed to decode health: %w", healthDecodeErr)
	}

	// Get index settings and mappings
	infoRes, infoErr := c.esClient.Indices.Get(
		[]string{indexName},
		c.esClient.Indices.Get.WithContext(ctx),
	)
	if infoErr != nil {
		return nil, fmt.Errorf("failed to get index info: %w", infoErr)
	}
	defer func() {
		_ = infoRes.Body.Close()
	}()

	if infoRes.IsError() {
		body, _ := io.ReadAll(infoRes.Body)
		return nil, fmt.Errorf("error getting index info: %s", string(body))
	}

	var infoData map[string]interface{}
	if infoDecodeErr := json.NewDecoder(infoRes.Body).Decode(&infoData); infoDecodeErr != nil {
		return nil, fmt.Errorf("failed to decode info: %w", infoDecodeErr)
	}

	indexData, ok := infoData[indexName].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid index data format")
	}

	// Extract document count
	docCount := extractDocumentCount(statsData, indexName)

	// Extract health and status
	health, status := extractHealthStatus(healthData, indexName)

	info := &IndexInfo{
		Name:          indexName,
		Health:        health,
		Status:        status,
		DocumentCount: docCount,
		Size:          "N/A", // Size calculation would require additional parsing
	}

	// Extract settings
	if settings, ok1 := indexData["settings"].(map[string]interface{}); ok1 {
		info.Settings = settings
	}

	// Extract mappings
	if mappings, ok1 := indexData["mappings"].(map[string]interface{}); ok1 {
		info.Mappings = mappings
	}

	return info, nil
}

// GetIndexHealth gets the health status of an index
func (c *Client) GetIndexHealth(ctx context.Context, indexName string) (string, error) {
	res, err := c.esClient.Cluster.Health(
		c.esClient.Cluster.Health.WithIndex(indexName),
		c.esClient.Cluster.Health.WithContext(ctx),
	)
	if err != nil {
		return "", fmt.Errorf("failed to get index health: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("error getting index health: %s", string(body))
	}

	var healthData map[string]interface{}
	if decodeErr := json.NewDecoder(res.Body).Decode(&healthData); decodeErr != nil {
		return "", fmt.Errorf("failed to decode health: %w", decodeErr)
	}

	if status, ok := healthData["status"].(string); ok {
		return status, nil
	}

	return unknownStatus, nil
}

// GetIndexMapping gets the mapping for an index
func (c *Client) GetIndexMapping(ctx context.Context, indexName string) (map[string]interface{}, error) {
	res, err := c.esClient.Indices.GetMapping(
		c.esClient.Indices.GetMapping.WithIndex(indexName),
		c.esClient.Indices.GetMapping.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get index mapping: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("error getting index mapping: %s", string(body))
	}

	var mappingData map[string]interface{}
	if decodeErr := json.NewDecoder(res.Body).Decode(&mappingData); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode mapping: %w", decodeErr)
	}

	if indexData, ok1 := mappingData[indexName].(map[string]interface{}); ok1 {
		if mappings, ok2 := indexData["mappings"].(map[string]interface{}); ok2 {
			return mappings, nil
		}
	}

	return nil, fmt.Errorf("mapping not found for index %s", indexName)
}

// UpdateIndexMapping updates the mapping for an index (additive only)
func (c *Client) UpdateIndexMapping(ctx context.Context, indexName string, mapping map[string]interface{}) error {
	// Elasticsearch only allows additive mapping updates
	// We need to use the put mapping API
	mappingJSON, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("failed to marshal mapping: %w", err)
	}

	res, err := c.esClient.Indices.PutMapping(
		[]string{indexName},
		strings.NewReader(string(mappingJSON)),
		c.esClient.Indices.PutMapping.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to update index mapping: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("error updating index mapping: %s", string(body))
	}

	return nil
}

// GetClusterHealth gets the overall cluster health
func (c *Client) GetClusterHealth(ctx context.Context) (map[string]interface{}, error) {
	res, err := c.esClient.Cluster.Health(c.esClient.Cluster.Health.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster health: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("error getting cluster health: %s", string(body))
	}

	var healthData map[string]interface{}
	if decodeErr := json.NewDecoder(res.Body).Decode(&healthData); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode cluster health: %w", decodeErr)
	}

	return healthData, nil
}
