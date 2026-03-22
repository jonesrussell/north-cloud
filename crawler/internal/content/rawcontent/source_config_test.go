//nolint:testpackage // Testing internal behavior requires same package access
package rawcontent

import (
	"context"
	"testing"

	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

type stubSources struct {
	configs []sources.Config
}

func (s stubSources) ValidateSourceByID(
	_ context.Context,
	_ string,
) (*configtypes.Source, error) {
	return &configtypes.Source{}, nil
}

func (s stubSources) GetSources() ([]sources.Config, error) {
	return s.configs, nil
}

func TestGetSourceConfigUsesConfiguredSourceName(t *testing.T) {
	svc := &RawContentService{
		logger: infralogger.NewNop(),
		sources: stubSources{
			configs: []sources.Config{
				{
					Name: "Sudbury.com",
					URL:  "https://www.sudbury.com",
				},
			},
		},
	}

	sourceName, _, _, _ := svc.getSourceConfig(
		"https://www.sudbury.com/news/local/story",
		"<html></html>",
	)

	if sourceName != "Sudbury.com" {
		t.Fatalf("expected configured source name, got %q", sourceName)
	}
}

func TestGetSourceConfigFallsBackToURLSourceNameWhenConfiguredNameIsEmpty(t *testing.T) {
	svc := &RawContentService{
		logger: infralogger.NewNop(),
		sources: stubSources{
			configs: []sources.Config{
				{
					Name: "",
					URL:  "https://www.sudbury.com",
				},
			},
		},
	}

	sourceName, _, _, _ := svc.getSourceConfig(
		"https://www.sudbury.com/news/local/story",
		"<html></html>",
	)

	if sourceName != "www_sudbury_com" {
		t.Fatalf("expected URL-derived fallback source name for empty configured name, got %q", sourceName)
	}
}

func TestGetSourceConfigFallsBackToURLSourceNameWhenSourceMissing(t *testing.T) {
	svc := &RawContentService{
		logger:  infralogger.NewNop(),
		sources: stubSources{},
	}

	sourceName, _, _, _ := svc.getSourceConfig(
		"https://www.sudbury.com/news/local/story",
		"<html></html>",
	)

	if sourceName != "www_sudbury_com" {
		t.Fatalf("expected URL-derived fallback source name, got %q", sourceName)
	}
}
