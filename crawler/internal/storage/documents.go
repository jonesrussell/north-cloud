package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jonesrussell/gocrawl/internal/domain"
)

// IndexDocument indexes a document in Elasticsearch
func (s *Storage) IndexDocument(ctx context.Context, index, id string, document any) error {
	if s.client == nil {
		return errors.New("elasticsearch client is not initialized")
	}

	ctx, cancel := s.createContextWithTimeout(ctx, DefaultIndexTimeout)
	defer cancel()

	body, err := json.Marshal(document)
	if err != nil {
		s.logger.Error("Failed to marshal document for indexing",
			"error", err,
			"index", index,
			"docID", id)
		return fmt.Errorf("failed to marshal document for indexing: %w", err)
	}

	res, err := s.client.Index(
		index,
		bytes.NewReader(body),
		s.client.Index.WithContext(ctx),
		s.client.Index.WithDocumentID(id),
		s.client.Index.WithRefresh("true"),
	)
	if err != nil {
		s.logger.Error("Failed to index document",
			"error", err,
			"index", index,
			"docID", id)
		return fmt.Errorf("failed to index document: %w", err)
	}

	if handleErr := s.handleResponse(res, "IndexDocument", "index", index, "docID", id); handleErr != nil {
		return handleErr
	}

	s.logger.Info("Document indexed successfully",
		"index", index,
		"docID", id,
		"type", fmt.Sprintf("%T", document),
		"url", getURLFromDocument(document))
	return nil
}

// getURLFromDocument extracts the URL from a document
func getURLFromDocument(doc any) string {
	switch v := doc.(type) {
	case *domain.Article:
		return v.Source
	case *domain.Content:
		return v.URL
	case *domain.Page:
		return v.URL
	default:
		return ""
	}
}

// GetDocument retrieves a document from Elasticsearch
func (s *Storage) GetDocument(ctx context.Context, index, id string, document any) error {
	ctx, cancel := s.createContextWithTimeout(ctx, DefaultIndexTimeout)
	defer cancel()

	res, err := s.client.Get(
		index,
		id,
		s.client.Get.WithContext(ctx),
	)
	if err != nil {
		s.logger.Error("Failed to get document",
			"error", err,
			"index", index,
			"docID", id)
		return fmt.Errorf("error getting document: %w", err)
	}

	if handleErr := s.handleResponse(res, "GetDocument", "index", index, "docID", id); handleErr != nil {
		return handleErr
	}

	if decodeErr := json.NewDecoder(res.Body).Decode(document); decodeErr != nil {
		s.logger.Error("Failed to decode document",
			"error", decodeErr,
			"index", index,
			"docID", id)
		return fmt.Errorf("error decoding document: %w", decodeErr)
	}

	return nil
}

// DeleteDocument deletes a document from Elasticsearch
func (s *Storage) DeleteDocument(ctx context.Context, index, docID string) error {
	ctx, cancel := s.createContextWithTimeout(ctx, DefaultIndexTimeout)
	defer cancel()

	res, err := s.client.Delete(
		index,
		docID,
		s.client.Delete.WithContext(ctx),
	)
	if err != nil {
		s.logger.Error("Failed to delete document", "error", err)
		return fmt.Errorf("error deleting document: %w", err)
	}

	if handleErr := s.handleResponse(res, "DeleteDocument", "index", index, "docID", docID); handleErr != nil {
		return handleErr
	}

	s.logger.Info("Deleted document", "index", index, "docID", docID)
	return nil
}
