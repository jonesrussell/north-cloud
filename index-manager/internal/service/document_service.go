package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jonesrussell/north-cloud/index-manager/internal/domain"
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// DocumentService provides business logic for document operations
type DocumentService struct {
	esClient     *elasticsearch.Client
	queryBuilder *elasticsearch.DocumentQueryBuilder
	logger       infralogger.Logger
}

// NewDocumentService creates a new document service
func NewDocumentService(esClient *elasticsearch.Client, logger infralogger.Logger) *DocumentService {
	return &DocumentService{
		esClient:     esClient,
		queryBuilder: elasticsearch.NewDocumentQueryBuilder(),
		logger:       logger,
	}
}

// QueryDocuments queries documents from an index with filters, pagination, and sorting.
func (s *DocumentService) QueryDocuments(
	ctx context.Context,
	indexName string,
	req *domain.DocumentQueryRequest,
) (*domain.DocumentQueryResponse, error) {
	// Verify index exists
	exists, err := s.esClient.IndexExists(ctx, indexName)
	if err != nil {
		return nil, fmt.Errorf("failed to check index existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("index %s does not exist", indexName)
	}

	// Build Elasticsearch query
	esQuery, err := s.queryBuilder.Build(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	s.logger.Debug("Querying documents",
		infralogger.String("index_name", indexName),
		infralogger.String("query", req.Query),
		infralogger.Int("page", req.Pagination.Page),
		infralogger.Int("size", req.Pagination.Size),
	)

	// Execute search
	res, err := s.esClient.SearchDocuments(ctx, indexName, esQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	// Parse response
	var esResponse struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				ID     string         `json:"_id"`
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if decodeErr := json.NewDecoder(res.Body).Decode(&esResponse); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", decodeErr)
	}

	// Convert to domain documents
	documents := make([]*domain.Document, 0, len(esResponse.Hits.Hits))
	for _, hit := range esResponse.Hits.Hits {
		doc := s.mapToDocument(hit.ID, hit.Source)
		documents = append(documents, doc)
	}

	totalHits := esResponse.Hits.Total.Value
	totalPages := int(math.Ceil(float64(totalHits) / float64(req.Pagination.Size)))

	return &domain.DocumentQueryResponse{
		Documents:   documents,
		TotalHits:   totalHits,
		TotalPages:  totalPages,
		CurrentPage: req.Pagination.Page,
		PageSize:    req.Pagination.Size,
	}, nil
}

// GetDocument retrieves a single document by ID
func (s *DocumentService) GetDocument(ctx context.Context, indexName, documentID string) (*domain.Document, error) {
	// Verify index exists
	exists, err := s.esClient.IndexExists(ctx, indexName)
	if err != nil {
		return nil, fmt.Errorf("failed to check index existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("index %s does not exist", indexName)
	}

	s.logger.Debug("Getting document",
		infralogger.String("index_name", indexName),
		infralogger.String("document_id", documentID),
	)

	// Get document from Elasticsearch
	source, err := s.esClient.GetDocument(ctx, indexName, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	return s.mapToDocument(documentID, source), nil
}

// UpdateDocument updates a document in an index
func (s *DocumentService) UpdateDocument(ctx context.Context, indexName, documentID string, doc *domain.Document) error {
	// Verify index exists
	exists, err := s.esClient.IndexExists(ctx, indexName)
	if err != nil {
		return fmt.Errorf("failed to check index existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("index %s does not exist", indexName)
	}

	s.logger.Info("Updating document",
		infralogger.String("index_name", indexName),
		infralogger.String("document_id", documentID),
	)

	// Convert document to map for update
	updateMap := s.documentToMap(doc)

	// Update document in Elasticsearch
	if updateErr := s.esClient.UpdateDocument(ctx, indexName, documentID, updateMap); updateErr != nil {
		return fmt.Errorf("failed to update document: %w", updateErr)
	}

	return nil
}

// DeleteDocument deletes a document from an index
func (s *DocumentService) DeleteDocument(ctx context.Context, indexName, documentID string) error {
	// Verify index exists
	exists, err := s.esClient.IndexExists(ctx, indexName)
	if err != nil {
		return fmt.Errorf("failed to check index existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("index %s does not exist", indexName)
	}

	s.logger.Info("Deleting document",
		infralogger.String("index_name", indexName),
		infralogger.String("document_id", documentID),
	)

	// Delete document from Elasticsearch
	if deleteErr := s.esClient.DeleteDocument(ctx, indexName, documentID); deleteErr != nil {
		return fmt.Errorf("failed to delete document: %w", deleteErr)
	}

	return nil
}

// BulkDeleteDocuments deletes multiple documents from an index
func (s *DocumentService) BulkDeleteDocuments(ctx context.Context, indexName string, documentIDs []string) error {
	if len(documentIDs) == 0 {
		return errors.New("no document IDs provided")
	}

	// Verify index exists
	exists, err := s.esClient.IndexExists(ctx, indexName)
	if err != nil {
		return fmt.Errorf("failed to check index existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("index %s does not exist", indexName)
	}

	s.logger.Info("Bulk deleting documents",
		infralogger.String("index_name", indexName),
		infralogger.Int("count", len(documentIDs)),
	)

	// Bulk delete documents from Elasticsearch
	if bulkErr := s.esClient.BulkDeleteDocuments(ctx, indexName, documentIDs); bulkErr != nil {
		return fmt.Errorf("failed to bulk delete documents: %w", bulkErr)
	}

	return nil
}

// mapToDocument converts Elasticsearch source map to domain Document
//
//nolint:gocognit // Complex mapping with many field extractions
func (s *DocumentService) mapToDocument(id string, source map[string]any) *domain.Document {
	doc := &domain.Document{
		ID:   id,
		Meta: make(map[string]any),
	}

	// Extract common fields
	if title, ok := source["title"].(string); ok {
		doc.Title = title
	}
	if url, ok := source["url"].(string); ok {
		doc.URL = url
	}
	if sourceName, ok := source["source_name"].(string); ok {
		doc.SourceName = sourceName
	}
	if contentType, ok := source["content_type"].(string); ok {
		doc.ContentType = contentType
	}
	if qualityScore, ok := source["quality_score"].(float64); ok {
		doc.QualityScore = int(qualityScore)
	}
	if body, ok := source["body"].(string); ok {
		doc.Body = body
	}
	if rawText, ok := source["raw_text"].(string); ok {
		doc.RawText = rawText
	}
	if rawHTML, ok := source["raw_html"].(string); ok {
		doc.RawHTML = rawHTML
	}

	// Extract topics array
	if topics, ok := source["topics"].([]any); ok {
		doc.Topics = make([]string, 0, len(topics))
		for _, topic := range topics {
			if topicStr, okTopic := topic.(string); okTopic {
				doc.Topics = append(doc.Topics, topicStr)
			}
		}
	}

	// Extract dates
	if publishedDateStr, ok := source["published_date"].(string); ok {
		if publishedDate, err := time.Parse(time.RFC3339, publishedDateStr); err == nil {
			doc.PublishedDate = &publishedDate
		}
	}
	if crawledAtStr, ok := source["crawled_at"].(string); ok {
		if crawledAt, err := time.Parse(time.RFC3339, crawledAtStr); err == nil {
			doc.CrawledAt = &crawledAt
		}
	}

	// Extract crime object
	doc.Crime = s.extractCrimeInfo(source)

	// Extract location object
	doc.Location = s.extractLocationInfo(source)

	// Compute is_crime_related for backward compatibility
	doc.IsCrimeRelated = doc.ComputedIsCrimeRelated()

	// Store remaining fields in Meta
	excludedKeys := map[string]bool{
		"title": true, "url": true, "source_name": true, "content_type": true,
		"quality_score": true, "body": true, "raw_text": true, "raw_html": true,
		"topics": true, "published_date": true, "crawled_at": true,
		"crime": true, "location": true, "is_crime_related": true,
	}
	for key, value := range source {
		if !excludedKeys[key] {
			doc.Meta[key] = value
		}
	}

	return doc
}

// extractCrimeInfo extracts crime classification from ES source
func (s *DocumentService) extractCrimeInfo(source map[string]any) *domain.CrimeInfo {
	crimeData, hasCrime := source["crime"].(map[string]any)
	if !hasCrime {
		// Fallback to legacy is_crime_related boolean
		if isCrime, hasBool := source["is_crime_related"].(bool); hasBool && isCrime {
			return &domain.CrimeInfo{Relevance: "core_street_crime"}
		}
		return nil
	}

	crime := &domain.CrimeInfo{}
	if v, hasSubLabel := crimeData["sub_label"].(string); hasSubLabel {
		crime.SubLabel = v
	}
	if v, hasPrimary := crimeData["primary_crime_type"].(string); hasPrimary {
		crime.PrimaryCrimeType = v
	}
	if v, hasRelevance := crimeData["relevance"].(string); hasRelevance {
		crime.Relevance = v
	}
	if v, hasConfidence := crimeData["final_confidence"].(float64); hasConfidence {
		crime.Confidence = v
	}
	if v, hasHomepage := crimeData["homepage_eligible"].(bool); hasHomepage {
		crime.HomepageEligible = v
	}
	if v, hasReview := crimeData["review_required"].(bool); hasReview {
		crime.ReviewRequired = v
	}
	if v, hasModel := crimeData["model_version"].(string); hasModel {
		crime.ModelVersion = v
	}

	// Extract crime_types array
	if types, hasTypes := crimeData["crime_types"].([]any); hasTypes {
		crime.CrimeTypes = make([]string, 0, len(types))
		for _, t := range types {
			if ts, isStr := t.(string); isStr {
				crime.CrimeTypes = append(crime.CrimeTypes, ts)
			}
		}
	}

	return crime
}

// extractLocationInfo extracts location data from ES source
func (s *DocumentService) extractLocationInfo(source map[string]any) *domain.LocationInfo {
	locData, hasLoc := source["location"].(map[string]any)
	if !hasLoc {
		return nil
	}

	loc := &domain.LocationInfo{}
	if v, hasCity := locData["city"].(string); hasCity {
		loc.City = v
	}
	if v, hasProvince := locData["province"].(string); hasProvince {
		loc.Province = v
	}
	if v, hasCountry := locData["country"].(string); hasCountry {
		loc.Country = v
	}
	if v, hasSpec := locData["specificity"].(string); hasSpec {
		loc.Specificity = v
	}
	if v, hasConf := locData["confidence"].(float64); hasConf {
		loc.Confidence = v
	}

	return loc
}

// documentToMap converts domain Document to map for Elasticsearch update
func (s *DocumentService) documentToMap(doc *domain.Document) map[string]any {
	result := make(map[string]any)

	if doc.Title != "" {
		result["title"] = doc.Title
	}
	if doc.URL != "" {
		result["url"] = doc.URL
	}
	if doc.SourceName != "" {
		result["source_name"] = doc.SourceName
	}
	if doc.ContentType != "" {
		result["content_type"] = doc.ContentType
	}
	if doc.QualityScore > 0 {
		result["quality_score"] = doc.QualityScore
	}
	if doc.Body != "" {
		result["body"] = doc.Body
	}
	if doc.RawText != "" {
		result["raw_text"] = doc.RawText
	}
	if doc.RawHTML != "" {
		result["raw_html"] = doc.RawHTML
	}
	if len(doc.Topics) > 0 {
		result["topics"] = doc.Topics
	}
	if doc.PublishedDate != nil {
		result["published_date"] = doc.PublishedDate.Format(time.RFC3339)
	}
	if doc.CrawledAt != nil {
		result["crawled_at"] = doc.CrawledAt.Format(time.RFC3339)
	}

	// Add crime object
	if doc.Crime != nil {
		result["crime"] = s.crimeInfoToMap(doc.Crime)
		result["is_crime_related"] = doc.Crime.IsCrimeRelated()
	} else {
		result["is_crime_related"] = doc.IsCrimeRelated
	}

	// Add location object
	if doc.Location != nil {
		result["location"] = s.locationInfoToMap(doc.Location)
	}

	// Merge meta fields
	for key, value := range doc.Meta {
		result[key] = value
	}

	return result
}

// crimeInfoToMap converts CrimeInfo to map for ES
func (s *DocumentService) crimeInfoToMap(crime *domain.CrimeInfo) map[string]any {
	result := make(map[string]any)
	if crime.SubLabel != "" {
		result["sub_label"] = crime.SubLabel
	}
	if crime.PrimaryCrimeType != "" {
		result["primary_crime_type"] = crime.PrimaryCrimeType
	}
	if crime.Relevance != "" {
		result["relevance"] = crime.Relevance
	}
	if len(crime.CrimeTypes) > 0 {
		result["crime_types"] = crime.CrimeTypes
	}
	if crime.Confidence > 0 {
		result["final_confidence"] = crime.Confidence
	}
	result["homepage_eligible"] = crime.HomepageEligible
	result["review_required"] = crime.ReviewRequired
	if crime.ModelVersion != "" {
		result["model_version"] = crime.ModelVersion
	}
	return result
}

// locationInfoToMap converts LocationInfo to map for ES
func (s *DocumentService) locationInfoToMap(loc *domain.LocationInfo) map[string]any {
	result := make(map[string]any)
	if loc.City != "" {
		result["city"] = loc.City
	}
	if loc.Province != "" {
		result["province"] = loc.Province
	}
	if loc.Country != "" {
		result["country"] = loc.Country
	}
	if loc.Specificity != "" {
		result["specificity"] = loc.Specificity
	}
	if loc.Confidence > 0 {
		result["confidence"] = loc.Confidence
	}
	return result
}
