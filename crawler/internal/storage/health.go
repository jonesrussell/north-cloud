package storage

import (
	"context"
	"encoding/json"
	"fmt"
)

// GetIndexHealth retrieves the health status of an index
func (s *Storage) GetIndexHealth(ctx context.Context, index string) (string, error) {
	res, err := s.client.Cluster.Health(
		s.client.Cluster.Health.WithContext(ctx),
		s.client.Cluster.Health.WithIndex(index),
	)
	if err != nil {
		return "", fmt.Errorf("failed to get index health: %w", err)
	}

	if handleErr := s.handleResponse(res, "GetIndexHealth", "index", index); handleErr != nil {
		return "", handleErr
	}

	var health map[string]any
	if decodeErr := json.NewDecoder(res.Body).Decode(&health); decodeErr != nil {
		return "", fmt.Errorf("error decoding index health: %w", decodeErr)
	}

	status, ok := health["status"].(string)
	if !ok {
		return "", ErrInvalidIndexHealth
	}

	return status, nil
}

// GetIndexDocCount retrieves the document count for an index
func (s *Storage) GetIndexDocCount(ctx context.Context, index string) (int64, error) {
	res, err := s.client.Count(
		s.client.Count.WithContext(ctx),
		s.client.Count.WithIndex(index),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to get document count: %w", err)
	}

	if handleErr := s.handleResponse(res, "GetIndexDocCount", "index", index); handleErr != nil {
		return 0, handleErr
	}

	var count map[string]any
	if decodeErr := json.NewDecoder(res.Body).Decode(&count); decodeErr != nil {
		return 0, fmt.Errorf("error decoding document count: %w", decodeErr)
	}

	countValue, ok := count["count"].(float64)
	if !ok {
		return 0, ErrInvalidDocCount
	}

	return int64(countValue), nil
}
