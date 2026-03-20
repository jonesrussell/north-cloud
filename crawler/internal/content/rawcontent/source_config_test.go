//nolint:testpackage // Tests unexported getSourceConfig helper directly.
package rawcontent

import (
	"context"
	"errors"
	"testing"

	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

var errValidateSourceByIDNotImplemented = errors.New("ValidateSourceByID not implemented in stubSources")

type stubSources struct {
	configs []sources.Config
}

func (s stubSources) ValidateSourceByID(
	_ context.Context,
	_ string,
) (*configtypes.Source, error) {
	return nil, errValidateSourceByIDNotImplemented
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

func TestGetSourceConfigFallsBackToURLSourceNameWhenConfiguredNameEmpty(t *testing.T) {
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
		t.Fatalf("expected URL-derived fallback source name when configured name is empty, got %q", sourceName)
	}
}
