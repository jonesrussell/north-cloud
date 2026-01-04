package sources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/apiclient"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/types"
	storagetypes "github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
)

// ValidateSource validates a source configuration and returns the validated source.
// It checks if the source exists and is properly configured.
// If sources haven't been loaded yet, it loads them from the API first.
// If the source is not found, it attempts to reload sources from the API and retries once.
// Note: Index creation is now handled by the raw content pipeline, not here.
func (s *Sources) ValidateSource(
	ctx context.Context,
	sourceName string,
	indexManager storagetypes.IndexManager,
) (*configtypes.Source, error) {
	// Load sources lazily if they haven't been loaded yet
	if err := s.ensureSourcesLoaded(); err != nil {
		return nil, fmt.Errorf("failed to load sources: %w", err)
	}

	// Try validation with current sources
	source, err := s.validateSourceInternal(sourceName)
	if err == nil {
		return source, nil
	}

	// If source not found and we have an API URL, try reloading sources and retry
	if s.apiURL != "" && strings.Contains(err.Error(), "source not found") {
		if s.logger != nil {
			s.logger.Debug("Source not found, reloading sources from API",
				"source_name", sourceName,
				"api_url", s.apiURL)
		}

		// Reload sources from API
		if reloadErr := s.reloadSources(); reloadErr != nil {
			if s.logger != nil {
				s.logger.Warn("Failed to reload sources from API",
					"error", reloadErr)
			}
			// Return original error if reload fails
			return nil, err
		}

		// Retry validation with reloaded sources
		return s.validateSourceInternal(sourceName)
	}

	return nil, err
}

// ValidateSourceByID validates a source configuration by ID and returns the validated source.
// This is more efficient than ValidateSource as it can fetch a single source from the API
// without loading all sources.
func (s *Sources) ValidateSourceByID(
	ctx context.Context,
	sourceID string,
	indexManager storagetypes.IndexManager,
) (*configtypes.Source, error) {
	if sourceID == "" {
		return nil, errors.New("source ID is required")
	}

	// First, try to find the source in cached sources (if already loaded)
	s.mu.RLock()
	if len(s.sources) > 0 {
		for i := range s.sources {
			if s.sources[i].ID == sourceID {
				sourceConfig := s.sources[i]
				s.mu.RUnlock()
				// Convert to configtypes.Source
				source := types.ConvertToConfigSource(&sourceConfig)
				return source, nil
			}
		}
	}
	s.mu.RUnlock()

	// If not found in cache, fetch directly from API
	if s.apiURL == "" {
		return nil, errors.New("API URL not configured")
	}

	// Create API client and fetch source by ID
	apiClient := apiclient.NewClient(apiclient.WithBaseURL(s.apiURL))
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
	source := types.ConvertToConfigSource(sourceConfig)

	// Note: Index creation is now handled by the raw content pipeline, not here.
	// The indexManager parameter is kept for interface compatibility but is no longer used here.

	return source, nil
}

// validateSourceInternal performs the actual source validation logic.
func (s *Sources) validateSourceInternal(
	sourceName string,
) (*configtypes.Source, error) {
	// Get all sources (with read lock)
	s.mu.RLock()
	sourceConfigs := make([]Config, len(s.sources))
	copy(sourceConfigs, s.sources)
	s.mu.RUnlock()

	// If no sources are configured, return an error
	if len(sourceConfigs) == 0 {
		return nil, errors.New("no sources configured")
	}

	// Find the requested source (case-insensitive match)
	var selectedSource *Config
	var availableNames []string
	for i := range sourceConfigs {
		availableNames = append(availableNames, sourceConfigs[i].Name)
		// Try exact match first
		if sourceConfigs[i].Name == sourceName {
			selectedSource = &sourceConfigs[i]
			break
		}
	}

	// If exact match not found, try case-insensitive match
	if selectedSource == nil {
		for i := range sourceConfigs {
			if strings.EqualFold(sourceConfigs[i].Name, sourceName) {
				selectedSource = &sourceConfigs[i]
				break
			}
		}
	}

	// If source not found, return an error with available sources
	if selectedSource == nil {
		return nil, fmt.Errorf("source not found: %s. Available sources: %v", sourceName, availableNames)
	}

	// Convert to configtypes.Source
	source := types.ConvertToConfigSource(selectedSource)

	// Note: Legacy article and page index creation has been removed.
	// The system now uses the raw content pipeline which creates {source}_raw_content indexes.
	// The indexManager parameter is kept for interface compatibility but is no longer used here.

	return source, nil
}
