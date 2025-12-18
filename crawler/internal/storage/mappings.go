package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// GetMapping retrieves the mapping for an index
func (s *Storage) GetMapping(ctx context.Context, index string) (map[string]any, error) {
	res, err := s.client.Indices.GetMapping(
		s.client.Indices.GetMapping.WithContext(ctx),
		s.client.Indices.GetMapping.WithIndex(index),
	)
	if err != nil {
		s.logger.Error("Failed to get mapping", "error", err)
		return nil, fmt.Errorf("failed to get mapping: %w", err)
	}

	if handleErr := s.handleResponse(res, "GetMapping", "index", index); handleErr != nil {
		return nil, handleErr
	}

	var mapping map[string]any
	if decodeErr := json.NewDecoder(res.Body).Decode(&mapping); decodeErr != nil {
		s.logger.Error("Failed to get mapping", "error", decodeErr)
		return nil, fmt.Errorf("error decoding mapping: %w", decodeErr)
	}

	s.logger.Info("Retrieved mapping", "index", index)
	return mapping, nil
}

// UpdateMapping updates the mapping for an index
func (s *Storage) UpdateMapping(ctx context.Context, index string, mapping map[string]any) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(mapping); err != nil {
		return fmt.Errorf("error encoding mapping: %w", err)
	}

	res, err := s.client.Indices.PutMapping(
		[]string{index},
		&buf,
		s.client.Indices.PutMapping.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to update mapping: %w", err)
	}

	if handleErr := s.handleResponse(res, "UpdateMapping", "index", index); handleErr != nil {
		return handleErr
	}

	return nil
}
