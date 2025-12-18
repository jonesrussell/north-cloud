package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// SearchDocuments performs a search query and decodes the result into the provided value
func (s *Storage) SearchDocuments(ctx context.Context, index string, query map[string]any, result any) error {
	if s.client == nil {
		return errors.New("elasticsearch client is not initialized")
	}

	// First check if the index exists
	exists, err := s.IndexExists(ctx, index)
	if err != nil {
		return fmt.Errorf("failed to check index existence: %w", err)
	}
	if !exists {
		s.logger.Error("Index not found", "index", index)
		return fmt.Errorf("%w: %s", ErrIndexNotFound, index)
	}

	ctx, cancel := s.createContextWithTimeout(ctx, DefaultSearchTimeout)
	defer cancel()

	body, err := marshalJSON(query)
	if err != nil {
		return fmt.Errorf("error marshaling search query: %w", err)
	}

	res, err := s.client.Search(
		s.client.Search.WithContext(ctx),
		s.client.Search.WithIndex(index),
		s.client.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return fmt.Errorf("error executing search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("search error: %s", res.String())
	}

	if decodeErr := json.NewDecoder(res.Body).Decode(result); decodeErr != nil {
		return fmt.Errorf("error decoding search response: %w", decodeErr)
	}

	return nil
}

// Search performs a search query
func (s *Storage) Search(ctx context.Context, index string, query any) ([]any, error) {
	if s.client == nil {
		return nil, errors.New("elasticsearch client is not initialized")
	}

	// First check if the index exists
	exists, err := s.IndexExists(ctx, index)
	if err != nil {
		return nil, fmt.Errorf("failed to check index existence: %w", err)
	}
	if !exists {
		s.logger.Error("Index not found", "index", index)
		return nil, fmt.Errorf("%w: %s", ErrIndexNotFound, index)
	}

	ctx, cancel := s.createContextWithTimeout(ctx, DefaultSearchTimeout)
	defer cancel()

	body, err := marshalJSON(query)
	if err != nil {
		return nil, fmt.Errorf("error marshaling search query: %w", err)
	}

	res, err := s.client.Search(
		s.client.Search.WithContext(ctx),
		s.client.Search.WithIndex(index),
		s.client.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return nil, fmt.Errorf("error executing search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("search error: %s", res.String())
	}

	var result map[string]any
	if decodeErr := json.NewDecoder(res.Body).Decode(&result); decodeErr != nil {
		return nil, decodeErr
	}

	hits, ok := result["hits"].(map[string]any)
	if !ok {
		return nil, errors.New("invalid response format: hits object not found")
	}

	hitsArray, ok := hits["hits"].([]any)
	if !ok {
		return nil, errors.New("invalid response format: hits array not found")
	}

	return hitsArray, nil
}

// Count returns the number of documents matching the query
func (s *Storage) Count(ctx context.Context, index string, query any) (int64, error) {
	if s.client == nil {
		return 0, errors.New("elasticsearch client is not initialized")
	}

	// First check if the index exists
	exists, err := s.IndexExists(ctx, index)
	if err != nil {
		return 0, fmt.Errorf("failed to check index existence: %w", err)
	}
	if !exists {
		s.logger.Error("Index not found", "index", index)
		return 0, fmt.Errorf("%w: %s", ErrIndexNotFound, index)
	}

	ctx, cancel := s.createContextWithTimeout(ctx, DefaultSearchTimeout)
	defer cancel()

	body, err := marshalJSON(query)
	if err != nil {
		return 0, fmt.Errorf("error marshaling count query: %w", err)
	}

	res, err := s.client.Count(
		s.client.Count.WithContext(ctx),
		s.client.Count.WithIndex(index),
		s.client.Count.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return 0, fmt.Errorf("error executing count: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return 0, fmt.Errorf("count error: %s", res.String())
	}

	var result map[string]any
	if decodeErr := json.NewDecoder(res.Body).Decode(&result); decodeErr != nil {
		return 0, fmt.Errorf("error decoding count response: %w", decodeErr)
	}

	count, ok := result["count"].(float64)
	if !ok {
		return 0, errors.New("invalid response format: count not found")
	}

	return int64(count), nil
}

// Aggregate performs an aggregation query
func (s *Storage) Aggregate(ctx context.Context, index string, aggs any) (any, error) {
	if s.client == nil {
		return nil, errors.New("elasticsearch client is not initialized")
	}

	// First check if the index exists
	exists, err := s.IndexExists(ctx, index)
	if err != nil {
		return nil, fmt.Errorf("failed to check index existence: %w", err)
	}
	if !exists {
		s.logger.Error("Index not found", "index", index)
		return nil, fmt.Errorf("%w: %s", ErrIndexNotFound, index)
	}

	ctx, cancel := s.createContextWithTimeout(ctx, DefaultSearchTimeout)
	defer cancel()

	body, err := marshalJSON(map[string]any{
		"size": 0,
		"aggs": aggs,
	})
	if err != nil {
		return nil, fmt.Errorf("error marshaling aggregation query: %w", err)
	}

	res, err := s.client.Search(
		s.client.Search.WithContext(ctx),
		s.client.Search.WithIndex(index),
		s.client.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return nil, fmt.Errorf("error executing aggregation: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("aggregation error: %s", res.String())
	}

	var result map[string]any
	if decodeErr := json.NewDecoder(res.Body).Decode(&result); decodeErr != nil {
		return nil, fmt.Errorf("error decoding aggregation response: %w", decodeErr)
	}

	aggregations, ok := result["aggregations"].(map[string]any)
	if !ok {
		return nil, errors.New("invalid response format: aggregations not found")
	}

	return aggregations, nil
}
