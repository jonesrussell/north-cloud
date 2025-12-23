package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jonesrussell/north-cloud/index-manager/internal/database"
	"github.com/jonesrussell/north-cloud/index-manager/internal/domain"
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch"
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch/mappings"
)

// IndexService provides business logic for index operations
type IndexService struct {
	esClient *elasticsearch.Client
	db       *database.Connection
	logger   Logger
}

// Logger interface for logging
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
}

// NewIndexService creates a new index service
func NewIndexService(esClient *elasticsearch.Client, db *database.Connection, logger Logger) *IndexService {
	return &IndexService{
		esClient: esClient,
		db:       db,
		logger:   logger,
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
			return nil, fmt.Errorf("source_name is required when index_name is not provided")
		}
		indexName = GenerateIndexName(req.SourceName, req.IndexType)
	}

	// Validate index type
	if !isValidIndexType(req.IndexType) {
		return nil, fmt.Errorf("invalid index type: %s", req.IndexType)
	}

	// Get mapping
	var mapping map[string]interface{}
	var err error
	if req.Mapping != nil {
		mapping = req.Mapping
	} else {
		mapping, err = mappings.GetMappingForType(string(req.IndexType))
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
		info, err := s.esClient.GetIndexInfo(ctx, indexName)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing index info: %w", err)
		}
		return s.indexInfoToDomain(info, req.IndexType, req.SourceName), nil
	}

	// Create index in Elasticsearch
	if err := s.esClient.CreateIndex(ctx, indexName, mapping); err != nil {
		return nil, fmt.Errorf("failed to create index in Elasticsearch: %w", err)
	}

	// Record migration
	migration := &database.MigrationHistory{
		IndexName:     indexName,
		ToVersion:     sql.NullString{String: "1.0.0", Valid: true},
		MigrationType: "create",
		Status:        "pending",
		CreatedAt:     time.Now(),
		CompletedAt:   sql.NullTime{},
	}
	if err := s.db.RecordMigration(ctx, migration); err != nil {
		s.logger.Warn("Failed to record migration", "error", err)
	}

	// Update migration status
	if err := s.db.UpdateMigrationStatus(ctx, migration.ID, "completed", ""); err != nil {
		s.logger.Warn("Failed to update migration status", "error", err)
	}

	// Save metadata
	metadata := &database.IndexMetadata{
		IndexName:      indexName,
		IndexType:      string(req.IndexType),
		SourceName:     sql.NullString{String: req.SourceName, Valid: req.SourceName != ""},
		MappingVersion: "1.0.0",
		Status:         "active",
	}
	if err := s.db.SaveIndexMetadata(ctx, metadata); err != nil {
		s.logger.Warn("Failed to save index metadata", "error", err)
	}

	// Get index info
	info, err := s.esClient.GetIndexInfo(ctx, indexName)
	if err != nil {
		return nil, fmt.Errorf("failed to get index info: %w", err)
	}

	s.logger.Info("Index created successfully", "index_name", indexName, "index_type", req.IndexType)

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
	if err := s.esClient.DeleteIndex(ctx, indexName); err != nil {
		return fmt.Errorf("failed to delete index from Elasticsearch: %w", err)
	}

	// Record migration
	migration := &database.MigrationHistory{
		IndexName:     indexName,
		MigrationType: "delete",
		Status:        "pending",
		CreatedAt:     time.Now(),
	}
	if err := s.db.RecordMigration(ctx, migration); err != nil {
		s.logger.Warn("Failed to record migration", "error", err)
	}

	// Update migration status
	if err := s.db.UpdateMigrationStatus(ctx, migration.ID, "completed", ""); err != nil {
		s.logger.Warn("Failed to update migration status", "error", err)
	}

	// Update metadata
	if err := s.db.DeleteIndexMetadata(ctx, indexName); err != nil {
		s.logger.Warn("Failed to update index metadata", "error", err)
	}

	s.logger.Info("Index deleted successfully", "index_name", indexName)

	return nil
}

// ListIndices lists all indices with optional filtering
func (s *IndexService) ListIndices(ctx context.Context, indexType string, sourceName string) ([]*domain.Index, error) {
	var indices []string
	var err error

	if sourceName != "" {
		// List by source
		normalized := NormalizeSourceName(sourceName)
		pattern := fmt.Sprintf("%s_*", normalized)
		indices, err = s.esClient.ListIndices(ctx, pattern)
	} else if indexType != "" {
		// List by type
		pattern := fmt.Sprintf("*_%s", indexType)
		indices, err = s.esClient.ListIndices(ctx, pattern)
	} else {
		// List all
		indices, err = s.esClient.ListIndices(ctx, "*")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list indices: %w", err)
	}

	var result []*domain.Index
	for _, indexName := range indices {
		info, err := s.esClient.GetIndexInfo(ctx, indexName)
		if err != nil {
			s.logger.Warn("Failed to get index info", "index_name", indexName, "error", err)
			continue
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

		result = append(result, s.indexInfoToDomain(info, indexType, sourceName))
	}

	return result, nil
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

	var results []*domain.Index
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
		if err := s.DeleteIndex(ctx, indexName); err != nil {
			s.logger.Warn("Failed to delete index", "index_name", indexName, "error", err)
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
	}

	var totalDocs int64
	for _, indexName := range indices {
		info, err := s.esClient.GetIndexInfo(ctx, indexName)
		if err != nil {
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

	// Get cluster health
	clusterHealth, err := s.esClient.GetClusterHealth(ctx)
	if err == nil {
		if status, ok := clusterHealth["status"].(string); ok {
			stats.ClusterHealth = status
		}
	}

	return stats, nil
}

// Helper functions

func isValidIndexType(indexType domain.IndexType) bool {
	switch indexType {
	case domain.IndexTypeRawContent, domain.IndexTypeClassifiedContent, domain.IndexTypeArticle, domain.IndexTypePage:
		return true
	default:
		return false
	}
}

func (s *IndexService) inferIndexTypeAndSource(indexName string) (domain.IndexType, string) {
	// Try to infer from index name pattern: {source}_{type}
	parts := strings.Split(indexName, "_")
	if len(parts) < 2 {
		return "", ""
	}

	// Last part should be the type
	lastPart := parts[len(parts)-1]
	var indexType domain.IndexType
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
		sourceName := strings.TrimSuffix(indexName, suffix)
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
