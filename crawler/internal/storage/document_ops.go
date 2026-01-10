package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	infralogger "github.com/north-cloud/infrastructure/logger"
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
		s.logOperationError("IndexDocument", index, id, err)
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
		s.logOperationError("IndexDocument", index, id, err)
		return fmt.Errorf("failed to index document: %w", err)
	}
	defer s.closeResponse(res, "IndexDocument", index, id)

	if res.IsError() {
		s.logger.Error("Elasticsearch returned error response",
			infralogger.String("error", res.String()),
			infralogger.String("index", index),
			infralogger.String("docID", id),
		)
		return fmt.Errorf("elasticsearch error: %s", res.String())
	}

	s.logger.Info("Document indexed successfully",
		infralogger.String("index", index),
		infralogger.String("docID", id),
		infralogger.String("type", fmt.Sprintf("%T", document)),
		infralogger.String("url", getURLFromDocument(document)),
	)
	return nil
}

// GetDocument retrieves a document from Elasticsearch
func (s *Storage) GetDocument(ctx context.Context, index, id string, document any) error {
	res, err := s.client.Get(
		index,
		id,
		s.client.Get.WithContext(ctx),
	)
	if err != nil {
		s.logOperationError("GetDocument", index, id, err)
		return fmt.Errorf("error getting document: %w", err)
	}
	defer s.closeResponse(res, "GetDocument", index, id)

	if res.IsError() {
		return fmt.Errorf("error getting document: %s", res.String())
	}

	if decodeErr := json.NewDecoder(res.Body).Decode(document); decodeErr != nil {
		s.logOperationError("GetDocument", index, id, decodeErr)
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
		s.logOperationError("DeleteDocument", index, docID, err)
		return fmt.Errorf("error deleting document: %w", err)
	}
	defer s.closeResponse(res, "DeleteDocument", index, docID)

	if res.IsError() {
		s.logger.Error("Failed to delete document",
			infralogger.String("error", res.String()),
			infralogger.String("index", index),
			infralogger.String("doc_id", docID),
		)
		return fmt.Errorf("error deleting document: %s", res.String())
	}

	s.logger.Info("Deleted document",
		infralogger.String("index", index),
		infralogger.String("docID", docID),
	)
	return nil
}
