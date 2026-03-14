// Package services provides business logic services for the source-manager.
package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	"github.com/jonesrussell/north-cloud/source-manager/internal/services/osrm"
)

const (
	secondsPerMinute   = 60
	metersPerKm        = 1000.0
	maxNearbyForMatrix = 100
)

// TravelTimeService computes and caches travel times between communities.
type TravelTimeService struct {
	osrmClient    *osrm.Client
	travelRepo    *repository.TravelTimeRepository
	communityRepo *repository.CommunityRepository
	logger        infralogger.Logger
}

// NewTravelTimeService creates a new TravelTimeService.
func NewTravelTimeService(
	osrmClient *osrm.Client,
	travelRepo *repository.TravelTimeRepository,
	communityRepo *repository.CommunityRepository,
	log infralogger.Logger,
) *TravelTimeService {
	return &TravelTimeService{
		osrmClient:    osrmClient,
		travelRepo:    travelRepo,
		communityRepo: communityRepo,
		logger:        log,
	}
}

// ComputeMatrix computes travel times from a community to all nearby communities.
// It uses cached results where available and calls OSRM for cache misses.
func (s *TravelTimeService) ComputeMatrix(
	ctx context.Context, communityID string, radiusKm float64, maxMinutes int, mode string,
) ([]repository.CommunityWithTravelTime, error) {
	origin, err := s.communityRepo.GetByID(ctx, communityID)
	if err != nil {
		return nil, fmt.Errorf("get origin community: %w", err)
	}
	if origin == nil {
		return nil, fmt.Errorf("community not found: %s", communityID)
	}
	if origin.Latitude == nil || origin.Longitude == nil {
		return nil, fmt.Errorf("community %s has no coordinates", communityID)
	}

	nearby, findErr := s.communityRepo.FindNearby(
		ctx, *origin.Latitude, *origin.Longitude, radiusKm, maxNearbyForMatrix,
	)
	if findErr != nil {
		return nil, fmt.Errorf("find nearby communities: %w", findErr)
	}

	results := make([]repository.CommunityWithTravelTime, 0, len(nearby))

	for i := range nearby {
		dest := &nearby[i]
		if dest.ID == communityID {
			continue
		}

		cwt, computeErr := s.computeSingle(ctx, origin, dest, mode)
		if computeErr != nil {
			s.logger.Warn("Failed to compute travel time",
				infralogger.String("origin", communityID),
				infralogger.String("dest", dest.ID),
				infralogger.Error(computeErr),
			)
			continue
		}

		if cwt.DurationMinutes <= float64(maxMinutes) {
			results = append(results, *cwt)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].DurationMinutes < results[j].DurationMinutes
	})

	return results, nil
}

// computeSingle checks cache first, then calls OSRM if needed.
func (s *TravelTimeService) computeSingle(
	ctx context.Context,
	origin *models.Community,
	dest *models.CommunityWithDistance,
	mode string,
) (*repository.CommunityWithTravelTime, error) {
	cached, cacheErr := s.travelRepo.GetCachedTravelTime(ctx, origin.ID, dest.ID, mode)
	if cacheErr != nil {
		return nil, fmt.Errorf("check cache: %w", cacheErr)
	}

	if cached != nil {
		return &repository.CommunityWithTravelTime{
			CommunityID:     dest.ID,
			Name:            dest.Name,
			Latitude:        SafeFloat(dest.Latitude),
			Longitude:       SafeFloat(dest.Longitude),
			DurationMinutes: float64(cached.DurationSeconds) / float64(secondsPerMinute),
			DistanceKm:      float64(cached.DistanceMeters) / metersPerKm,
			TransportMode:   mode,
		}, nil
	}

	if dest.Latitude == nil || dest.Longitude == nil {
		return nil, fmt.Errorf("destination %s has no coordinates", dest.ID)
	}

	result, osrmErr := s.osrmClient.GetTravelTime(
		ctx,
		*origin.Latitude, *origin.Longitude,
		*dest.Latitude, *dest.Longitude,
		mode,
	)
	if osrmErr != nil {
		return nil, fmt.Errorf("OSRM travel time: %w", osrmErr)
	}

	s.cacheResult(ctx, origin.ID, dest.ID, mode, result)

	return &repository.CommunityWithTravelTime{
		CommunityID:     dest.ID,
		Name:            dest.Name,
		Latitude:        SafeFloat(dest.Latitude),
		Longitude:       SafeFloat(dest.Longitude),
		DurationMinutes: float64(result.DurationSeconds) / float64(secondsPerMinute),
		DistanceKm:      float64(result.DistanceMeters) / metersPerKm,
		TransportMode:   mode,
	}, nil
}

// cacheResult persists an OSRM result to the travel_time_cache table.
func (s *TravelTimeService) cacheResult(
	ctx context.Context, originID, destID, mode string, result *osrm.TravelTimeResult,
) {
	entry := repository.TravelTimeEntry{
		OriginCommunityID:      originID,
		DestinationCommunityID: destID,
		TransportMode:          mode,
		DurationSeconds:        result.DurationSeconds,
		DistanceMeters:         result.DistanceMeters,
		ComputedAt:             time.Now(),
	}
	if upsertErr := s.travelRepo.UpsertTravelTime(ctx, entry); upsertErr != nil {
		s.logger.Warn("Failed to cache travel time",
			infralogger.String("origin", originID),
			infralogger.String("dest", destID),
			infralogger.Error(upsertErr),
		)
	}
}

// SafeFloat dereferences a float64 pointer, returning 0 if nil.
func SafeFloat(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}
