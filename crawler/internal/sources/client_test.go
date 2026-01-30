package sources_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
)

func TestSourceClient_Interface(t *testing.T) {
	t.Helper()

	// Verify interface is defined
	var _ sources.Client = (*sources.HTTPClient)(nil)
}

func TestSource_HasRequiredFields(t *testing.T) {
	t.Helper()

	s := sources.Source{
		ID:        uuid.New(),
		Name:      "Test",
		URL:       "https://example.com",
		RateLimit: 10,
		MaxDepth:  2,
		Enabled:   true,
		Priority:  "normal",
	}

	if s.ID == uuid.Nil {
		t.Error("ID should not be nil")
	}
	if s.Name == "" {
		t.Error("Name should not be empty")
	}
	if s.URL == "" {
		t.Error("URL should not be empty")
	}
	if s.RateLimit == 0 {
		t.Error("RateLimit should not be zero")
	}
	if s.MaxDepth == 0 {
		t.Error("MaxDepth should not be zero")
	}
	if !s.Enabled {
		t.Error("Enabled should be true")
	}
	if s.Priority == "" {
		t.Error("Priority should not be empty")
	}
}

func TestNoOpClient_ReturnsNotFoundError(t *testing.T) {
	t.Helper()

	client := sources.NewNoOpClient()
	source, err := client.GetSource(context.Background(), uuid.New())

	if !errors.Is(err, sources.ErrSourceNotFound) {
		t.Errorf("expected ErrSourceNotFound, got %v", err)
	}
	if source != nil {
		t.Error("expected nil source from NoOpClient")
	}
}
