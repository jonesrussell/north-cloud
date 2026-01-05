package sources

import (
	"context"
	"errors"
	"fmt"

	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/apiclient"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/types"
)

// getAPIClient creates and returns an API client for the sources API.
func (s *Sources) getAPIClient() (*apiclient.Client, error) {
	if s.apiURL == "" {
		return nil, errors.New("API URL not configured")
	}
	opts := []apiclient.Option{apiclient.WithBaseURL(s.apiURL)}
	if s.jwtSecret != "" {
		opts = append(opts, apiclient.WithJWTSecret(s.jwtSecret))
	}
	return apiclient.NewClient(opts...), nil
}

// ValidateSourceByID validates a source configuration by ID and returns the validated source.
// Fetches the source directly from the API.
func (s *Sources) ValidateSourceByID(
	ctx context.Context,
	sourceID string,
) (*configtypes.Source, error) {
	if sourceID == "" {
		return nil, errors.New("source ID is required")
	}

	apiClient, err := s.getAPIClient()
	if err != nil {
		return nil, err
	}

	apiSource, err := apiClient.GetSource(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source from API: %w", err)
	}

	// Convert API source to SourceConfig
	sourceConfig, err := apiclient.ConvertAPISourceToConfig(apiSource)
	if err != nil {
		return nil, fmt.Errorf("failed to convert source: %w", err)
	}

	// Convert to configtypes.Source
	return types.ConvertToConfigSource(sourceConfig), nil
}
