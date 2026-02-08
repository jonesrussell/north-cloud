package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/jonesrussell/north-cloud/index-manager/internal/config"
	"github.com/jonesrussell/north-cloud/index-manager/internal/database"
	"github.com/jonesrussell/north-cloud/index-manager/internal/domain"
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch"
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch/mappings"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// IndexService provides business logic for index operations
type IndexService struct {
	esClient   *elasticsearch.Client
	db         *database.Connection
	logger     infralogger.Logger
	indexTypes config.IndexTypesConfig
}

// NewIndexService creates a new index service
func NewIndexService(
	esClient *elasticsearch.Client, db *database.Connection,
	logger infralogger.Logger, indexTypes config.IndexTypesConfig,
) *IndexService {
	return &IndexService{
		esClient:   esClient,
		db:         db,
		logger:     logger,
		indexTypes: indexTypes,
	}
}

// NormalizeSourceName normalizes a source name for index naming
func NormalizeSourceName(sourceName string) string {
	// Remove protocol if present
	sourceName = strings.TrimPrefix(sourceName, "http://")
	sourceName = strings.TrimPrefix(sourceName, "https://")

	// Replace dots and hyphens with underscores
	sourceName = strings.ReplaceAll(sourceName, ".", "_")
	sourceName = strings.ReplaceAll(sourceName, "-", "_")

	// Convert to lowercase
	return strings.ToLower(sourceName)
}

// GenerateIndexName generates an index name from source name and type
func GenerateIndexName(sourceName string, indexType domain.IndexType) string {
	normalized := NormalizeSourceName(sourceName)
	suffix := getIndexSuffix(indexType)
	return fmt.Sprintf("%s%s", normalized, suffix)
}

// getIndexSuffix returns the suffix for an index type
func getIndexSuffix(indexType domain.IndexType) string {
	switch indexType {
	case domain.IndexTypeRawContent:
		return "_raw_content"
	case domain.IndexTypeClassifiedContent:
		return "_classified_content"
	case domain.IndexTypeArticle:
		return "_articles"
	case domain.IndexTypePage:
		return "_pages"
	default:
		return ""
	}
}

// CreateIndex creates an index with validation and metadata tracking
func (s *IndexService) CreateIndex(ctx context.Context, req *domain.CreateIndexRequest) (*domain.Index, error) {
	// Generate index name if not provided
	indexName := req.IndexName
	if indexName == "" {
		if req.SourceName == "" {
			return nil, errors.New("source_name is required when index_name is not provided")
		}
		indexName = GenerateIndexName(req.SourceName, req.IndexType)
	}

	// Validate index type
	if !isValidIndexType(req.IndexType) {
		return nil, fmt.Errorf("invalid index type: %s", req.IndexType)
	}

	// Get mapping
	var mapping map[string]any
	var err error
	if req.Mapping != nil {
		mapping = req.Mapping
	} else {
		mapping, err = mappings.GetMappingForType(string(req.IndexType), s.getShards(req.IndexType), s.getReplicas(req.IndexType))
		if err != nil {
			return nil, fmt.Errorf("failed to get mapping for type %s: %w", req.IndexType, err)
		}
	}

	// Check if index already exists
	exists, err := s.esClient.IndexExists(ctx, indexName)
	if err != nil {
		return nil, fmt.Errorf("failed to check if index exists: %w", err)
	}
	if exists {
		// Return existing index info
		info, infoErr := s.esClient.GetIndexInfo(ctx, indexName)
		if infoErr != nil {
			return nil, fmt.Errorf("failed to get existing index info: %w", infoErr)
		}
		return s.indexInfoToDomain(info, req.IndexType, req.SourceName), nil
	}

	// Create index in Elasticsearch
	if createErr := s.esClient.CreateIndex(ctx, indexName, mapping); createErr != nil {
		return nil, fmt.Errorf("failed to create index in Elasticsearch: %w", createErr)
	}

	// Record migration
	migration := &database.MigrationHistory{
		IndexName:     indexName,
		ToVersion:     sql.NullString{String: mappings.GetMappingVersion(string(req.IndexType)), Valid: true},
		MigrationType: "create",
		Status:        "pending",
		CreatedAt:     time.Now(),
		CompletedAt:   sql.NullTime{},
	}
	if recordErr := s.db.RecordMigration(ctx, migration); recordErr != nil {
		s.logger.Warn("Failed to record migration", infralogger.Error(recordErr))
	}

	// Update migration status
	if updateErr := s.db.UpdateMigrationStatus(ctx, migration.ID, "completed", ""); updateErr != nil {
		s.logger.Warn("Failed to update migration status", infralogger.Error(updateErr))
	}

	// Save metadata
	metadata := &database.IndexMetadata{
		IndexName:      indexName,
		IndexType:      string(req.IndexType),
		SourceName:     sql.NullString{String: req.SourceName, Valid: req.SourceName != ""},
		MappingVersion: mappings.GetMappingVersion(string(req.IndexType)),
		Status:         "active",
	}
	if saveErr := s.db.SaveIndexMetadata(ctx, metadata); saveErr != nil {
		s.logger.Warn("Failed to save index metadata", infralogger.Error(saveErr))
	}

	// Get index info
	info, err := s.esClient.GetIndexInfo(ctx, indexName)
	if err != nil {
		return nil, fmt.Errorf("failed to get index info: %w", err)
	}

	s.logger.Info("Index created successfully",
		infralogger.String("index_name", indexName),
		infralogger.String("index_type", string(req.IndexType)),
	)

	return s.indexInfoToDomain(info, req.IndexType, req.SourceName), nil
}

// DeleteIndex deletes an index and updates metadata
func (s *IndexService) DeleteIndex(ctx context.Context, indexName string) error {
	// Check if index exists
	exists, err := s.esClient.IndexExists(ctx, indexName)
	if err != nil {
		return fmt.Errorf("failed to check if index exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("index %s does not exist", indexName)
	}

	// Delete from Elasticsearch
	if deleteErr := s.esClient.DeleteIndex(ctx, indexName); deleteErr != nil {
		return fmt.Errorf("failed to delete index from Elasticsearch: %w", deleteErr)
	}

	// Record migration
	migration := &database.MigrationHistory{
		IndexName:     indexName,
		MigrationType: "delete",
		Status:        "pending",
		CreatedAt:     time.Now(),
	}
	if recordErr := s.db.RecordMigration(ctx, migration); recordErr != nil {
		s.logger.Warn("Failed to record migration", infralogger.Error(recordErr))
	}

	// Update migration status
	if updateErr := s.db.UpdateMigrationStatus(ctx, migration.ID, "completed", ""); updateErr != nil {
		s.logger.Warn("Failed to update migration status", infralogger.Error(updateErr))
	}

	// Update metadata
	if deleteMetaErr := s.db.DeleteIndexMetadata(ctx, indexName); deleteMetaErr != nil {
		s.logger.Warn("Failed to update index metadata", infralogger.Error(deleteMetaErr))
	}

	s.logger.Info("Index deleted successfully", infralogger.String("index_name", indexName))

	return nil
}

// ListIndices lists all indices with pagination, filtering, and sorting
func (s *IndexService) ListIndices(ctx context.Context, req *domain.ListIndicesRequest) (*domain.ListIndicesResponse, error) {
	var indices []string
	var err error

	if req.SourceName != "" {
		// List by source
		normalized := NormalizeSourceName(req.SourceName)
		pattern := fmt.Sprintf("%s_*", normalized)
		indices, err = s.esClient.ListIndices(ctx, pattern)
	} else if req.Type != "" {
		// List by type
		pattern := fmt.Sprintf("*_%s", req.Type)
		indices, err = s.esClient.ListIndices(ctx, pattern)
	} else {
		// List all
		indices, err = s.esClient.ListIndices(ctx, "*")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list indices: %w", err)
	}

	result := make([]*domain.Index, 0, len(indices))
	for _, indexName := range indices {
		info, infoErr := s.esClient.GetIndexInfo(ctx, indexName)
		if infoErr != nil {
			s.logger.Warn("Failed to get index info",
				infralogger.String("index_name", indexName),
				infralogger.Error(infoErr),
			)
			continue
		}

		// Get metadata
		metadata, _ := s.db.GetIndexMetadata(ctx, indexName)
		var idxType domain.IndexType
		var inferredSourceName string
		if metadata != nil {
			idxType = domain.IndexType(metadata.IndexType)
			if metadata.SourceName.Valid {
				inferredSourceName = metadata.SourceName.String
			}
		} else {
			// Try to infer from index name
			idxType, inferredSourceName = s.inferIndexTypeAndSource(indexName)
		}

		result = append(result, s.indexInfoToDomain(info, idxType, inferredSourceName))
	}

	// Apply search filter
	if req.Search != "" {
		result = filterBySearch(result, req.Search)
	}

	// Apply health filter
	if req.Health != "" {
		result = filterByHealth(result, req.Health)
	}

	// Get total before pagination
	total := len(result)

	// Apply sorting
	sortIndices(result, req.SortBy, req.SortOrder)

	// Apply pagination
	result = paginateIndices(result, req.Offset, req.Limit)

	return &domain.ListIndicesResponse{
		Indices: result,
		Total:   total,
		Limit:   req.Limit,
		Offset:  req.Offset,
	}, nil
}

// filterBySearch filters indices by name (case-insensitive substring match)
func filterBySearch(indices []*domain.Index, search string) []*domain.Index {
	search = strings.ToLower(search)
	filtered := make([]*domain.Index, 0, len(indices))
	for _, idx := range indices {
		if strings.Contains(strings.ToLower(idx.Name), search) {
			filtered = append(filtered, idx)
		}
	}
	return filtered
}

// filterByHealth filters indices by health status
func filterByHealth(indices []*domain.Index, health string) []*domain.Index {
	filtered := make([]*domain.Index, 0, len(indices))
	for _, idx := range indices {
		if strings.EqualFold(idx.Health, health) {
			filtered = append(filtered, idx)
		}
	}
	return filtered
}

// Health order constants for sorting
const (
	healthOrderGreen  = 0
	healthOrderYellow = 1
	healthOrderRed    = 2
)

// sortIndices sorts indices by specified field and order
func sortIndices(indices []*domain.Index, sortBy, sortOrder string) {
	sort.Slice(indices, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "document_count":
			less = indices[i].DocumentCount < indices[j].DocumentCount
		case "size":
			// Size is string like "1.2kb" - compare as strings for simplicity
			less = indices[i].Size < indices[j].Size
		case "health":
			// Health priority: green < yellow < red
			healthOrder := map[string]int{
				"green":  healthOrderGreen,
				"yellow": healthOrderYellow,
				"red":    healthOrderRed,
			}
			less = healthOrder[indices[i].Health] < healthOrder[indices[j].Health]
		case "type":
			less = string(indices[i].Type) < string(indices[j].Type)
		default: // name
			less = indices[i].Name < indices[j].Name
		}

		if sortOrder == "desc" {
			return !less
		}
		return less
	})
}

// paginateIndices slices indices array for pagination
func paginateIndices(indices []*domain.Index, offset, limit int) []*domain.Index {
	if offset >= len(indices) {
		return []*domain.Index{}
	}

	end := offset + limit
	if end > len(indices) {
		end = len(indices)
	}

	return indices[offset:end]
}

// GetIndex gets detailed information about an index
func (s *IndexService) GetIndex(ctx context.Context, indexName string) (*domain.Index, error) {
	info, err := s.esClient.GetIndexInfo(ctx, indexName)
	if err != nil {
		return nil, fmt.Errorf("failed to get index info: %w", err)
	}

	// Get metadata
	metadata, _ := s.db.GetIndexMetadata(ctx, indexName)
	var indexType domain.IndexType
	var sourceName string
	if metadata != nil {
		indexType = domain.IndexType(metadata.IndexType)
		if metadata.SourceName.Valid {
			sourceName = metadata.SourceName.String
		}
	} else {
		// Try to infer from index name
		indexType, sourceName = s.inferIndexTypeAndSource(indexName)
	}

	return s.indexInfoToDomain(info, indexType, sourceName), nil
}

// CreateIndexesForSource creates all indexes for a source
func (s *IndexService) CreateIndexesForSource(ctx context.Context, sourceName string, indexTypes []domain.IndexType) ([]*domain.Index, error) {
	if len(indexTypes) == 0 {
		// Default to raw_content and classified_content
		indexTypes = []domain.IndexType{domain.IndexTypeRawContent, domain.IndexTypeClassifiedContent}
	}

	results := make([]*domain.Index, 0, len(indexTypes))
	for _, indexType := range indexTypes {
		req := &domain.CreateIndexRequest{
			IndexType:  indexType,
			SourceName: sourceName,
		}
		index, err := s.CreateIndex(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to create %s index for source %s: %w", indexType, sourceName, err)
		}
		results = append(results, index)
	}

	return results, nil
}

// DeleteIndexesForSource deletes all indexes for a source
func (s *IndexService) DeleteIndexesForSource(ctx context.Context, sourceName string) error {
	normalized := NormalizeSourceName(sourceName)
	pattern := fmt.Sprintf("%s_*", normalized)

	indices, err := s.esClient.ListIndices(ctx, pattern)
	if err != nil {
		return fmt.Errorf("failed to list indices for source: %w", err)
	}

	for _, indexName := range indices {
		if deleteErr := s.DeleteIndex(ctx, indexName); deleteErr != nil {
			s.logger.Warn("Failed to delete index",
				infralogger.String("index_name", indexName),
				infralogger.Error(deleteErr),
			)
			// Continue with other indexes
		}
	}

	return nil
}

// GetIndexHealth gets the health status of an index
func (s *IndexService) GetIndexHealth(ctx context.Context, indexName string) (string, error) {
	return s.esClient.GetIndexHealth(ctx, indexName)
}

// GetStats gets statistics about all indexes
func (s *IndexService) GetStats(ctx context.Context) (*domain.IndexStats, error) {
	indices, err := s.esClient.ListIndices(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("failed to list indices: %w", err)
	}

	stats := &domain.IndexStats{
		TotalIndexes:    len(indices),
		IndexesByType:   make(map[string]int),
		IndexesByHealth: make(map[string]int),
		IndexedToday:    0,
	}

	var totalDocs int64
	for _, indexName := range indices {
		info, infoErr := s.esClient.GetIndexInfo(ctx, indexName)
		if infoErr != nil {
			continue
		}

		totalDocs += info.DocumentCount

		// Count by type
		metadata, _ := s.db.GetIndexMetadata(ctx, indexName)
		if metadata != nil {
			stats.IndexesByType[metadata.IndexType]++
		}

		// Count by health
		stats.IndexesByHealth[info.Health]++
	}

	stats.TotalDocuments = totalDocs

	// Get today's indexed count from all raw_content indexes
	indexedToday, err := s.getIndexedTodayCount(ctx)
	if err != nil {
		s.logger.Warn("Failed to get today's indexed count",
			infralogger.Error(err),
		)
		// Continue with 0 if query fails
		stats.IndexedToday = 0
	} else {
		stats.IndexedToday = indexedToday
	}

	// Get cluster health
	clusterHealth, err := s.esClient.GetClusterHealth(ctx)
	if err == nil {
		if status, ok := clusterHealth["status"].(string); ok {
			stats.ClusterHealth = status
		}
	}

	return stats, nil
}

// getIndexedTodayCount counts documents indexed today across all raw_content indexes
func (s *IndexService) getIndexedTodayCount(ctx context.Context) (int64, error) {
	// Get today's start time (00:00:00)
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Build query to count documents indexed today
	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"filter": []any{
					map[string]any{
						"range": map[string]any{
							"crawled_at": map[string]any{
								"gte": todayStart.Format(time.RFC3339),
							},
						},
					},
				},
			},
		},
		"size": 0, // We only need the count
	}

	// Search all raw_content indexes
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal query: %w", err)
	}

	res, err := s.esClient.GetClient().Search(
		s.esClient.GetClient().Search.WithContext(ctx),
		s.esClient.GetClient().Search.WithIndex("*_raw_content"),
		s.esClient.GetClient().Search.WithBody(strings.NewReader(string(queryJSON))),
		s.esClient.GetClient().Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to execute search: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return 0, fmt.Errorf("search returned error [%d]: %s", res.StatusCode, string(body))
	}

	// Parse response to get total count
	var esResponse struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
		} `json:"hits"`
	}

	if decodeErr := json.NewDecoder(res.Body).Decode(&esResponse); decodeErr != nil {
		return 0, fmt.Errorf("failed to decode search response: %w", decodeErr)
	}

	return esResponse.Hits.Total.Value, nil
}

// Helper functions

func (s *IndexService) getShards(indexType domain.IndexType) int {
	switch indexType {
	case domain.IndexTypeRawContent:
		return s.indexTypes.RawContent.Shards
	case domain.IndexTypeClassifiedContent:
		return s.indexTypes.ClassifiedContent.Shards
	case domain.IndexTypeArticle, domain.IndexTypePage:
		return 1
	default:
		return 1
	}
}

func (s *IndexService) getReplicas(indexType domain.IndexType) int {
	switch indexType {
	case domain.IndexTypeRawContent:
		return s.indexTypes.RawContent.Replicas
	case domain.IndexTypeClassifiedContent:
		return s.indexTypes.ClassifiedContent.Replicas
	case domain.IndexTypeArticle, domain.IndexTypePage:
		return 1
	default:
		return 1
	}
}

func isValidIndexType(indexType domain.IndexType) bool {
	switch indexType {
	case domain.IndexTypeRawContent, domain.IndexTypeClassifiedContent, domain.IndexTypeArticle, domain.IndexTypePage:
		return true
	default:
		return false
	}
}

func (s *IndexService) inferIndexTypeAndSource(indexName string) (indexType domain.IndexType, sourceName string) {
	// Try to infer from index name pattern: {source}_{type}
	const minIndexNameParts = 2
	parts := strings.Split(indexName, "_")
	if len(parts) < minIndexNameParts {
		return "", ""
	}

	// Last part should be the type
	lastPart := parts[len(parts)-1]
	switch lastPart {
	case "raw", "content":
		if strings.Contains(indexName, "raw_content") {
			indexType = domain.IndexTypeRawContent
		} else if strings.Contains(indexName, "classified_content") {
			indexType = domain.IndexTypeClassifiedContent
		}
	case "articles":
		indexType = domain.IndexTypeArticle
	case "pages":
		indexType = domain.IndexTypePage
	}

	// Source name is everything before the type suffix
	if indexType != "" {
		suffix := getIndexSuffix(indexType)
		sourceName = strings.TrimSuffix(indexName, suffix)
		return indexType, sourceName
	}

	return "", ""
}

func (s *IndexService) indexInfoToDomain(info *elasticsearch.IndexInfo, indexType domain.IndexType, sourceName string) *domain.Index {
	return &domain.Index{
		Name:          info.Name,
		Type:          indexType,
		SourceName:    sourceName,
		Health:        info.Health,
		Status:        info.Status,
		DocumentCount: info.DocumentCount,
		Size:          info.Size,
	}
}
