// Package testhelpers provides shared test utilities for the classifier service.
package testhelpers

import (
	"context"
	"errors"
	"sync"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

// ErrSourceNotFound is returned when a source is not found in the mock database.
var ErrSourceNotFound = errors.New("source not found")

// MockSourceReputationDB implements SourceReputationDB for testing.
type MockSourceReputationDB struct {
	mu      sync.RWMutex
	sources map[string]*domain.SourceReputation
}

// NewMockSourceReputationDB creates a new mock database.
func NewMockSourceReputationDB() *MockSourceReputationDB {
	return &MockSourceReputationDB{
		sources: make(map[string]*domain.SourceReputation),
	}
}

// GetSource retrieves a source by name.
func (m *MockSourceReputationDB) GetSource(_ context.Context, sourceName string) (*domain.SourceReputation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if source, ok := m.sources[sourceName]; ok {
		return source, nil
	}
	return nil, ErrSourceNotFound
}

// CreateSource creates a new source.
func (m *MockSourceReputationDB) CreateSource(_ context.Context, source *domain.SourceReputation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sources[source.SourceName] = source
	return nil
}

// UpdateSource updates an existing source.
func (m *MockSourceReputationDB) UpdateSource(_ context.Context, source *domain.SourceReputation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sources[source.SourceName] = source
	return nil
}

// GetOrCreateSource retrieves or creates a source.
func (m *MockSourceReputationDB) GetOrCreateSource(_ context.Context, sourceName string) (*domain.SourceReputation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if source, ok := m.sources[sourceName]; ok {
		return source, nil
	}
	newSource := &domain.SourceReputation{
		SourceName:    sourceName,
		TotalArticles: 0,
	}
	m.sources[sourceName] = newSource
	return newSource, nil
}

// SetSource sets a source directly (for test setup).
func (m *MockSourceReputationDB) SetSource(source *domain.SourceReputation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sources[source.SourceName] = source
}
