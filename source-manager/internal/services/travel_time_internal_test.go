package services

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	"github.com/jonesrussell/north-cloud/source-manager/internal/services/osrm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger(t *testing.T) infralogger.Logger {
	t.Helper()
	log, err := infralogger.New(infralogger.Config{
		Level:       "debug",
		Format:      "json",
		Development: false,
	})
	require.NoError(t, err)
	return log
}

func TestCacheResult_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	log := testLogger(t)
	travelRepo := repository.NewTravelTimeRepository(db, log)
	svc := NewTravelTimeService(nil, travelRepo, nil, log)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO travel_time_cache")).
		WithArgs("origin-1", "dest-1", "driving",
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	result := &osrm.TravelTimeResult{
		DurationSeconds: 3600,
		DistanceMeters:  50000,
	}

	// Should not panic even though it's fire-and-forget
	svc.cacheResult(context.Background(), "origin-1", "dest-1", "driving", result)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCacheResult_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	log := testLogger(t)
	travelRepo := repository.NewTravelTimeRepository(db, log)
	svc := NewTravelTimeService(nil, travelRepo, nil, log)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO travel_time_cache")).
		WillReturnError(errors.New("db error"))

	result := &osrm.TravelTimeResult{
		DurationSeconds: 100,
		DistanceMeters:  1000,
	}

	// Should log warning but not panic
	svc.cacheResult(context.Background(), "origin-1", "dest-1", "walking", result)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestComputeSingle_CacheHit(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	log := testLogger(t)
	travelRepo := repository.NewTravelTimeRepository(db, log)
	svc := NewTravelTimeService(nil, travelRepo, nil, log)

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, origin_community_id")).
		WithArgs("origin-1", "dest-1", "driving").
		WillReturnRows(
			sqlmock.NewRows([]string{
				"id", "origin_community_id", "destination_community_id",
				"transport_mode", "duration_seconds", "distance_meters", "computed_at",
			}).AddRow(
				1, "origin-1", "dest-1",
				"driving", 1800, 30000, now,
			),
		)

	originLat, originLon := 46.49, -81.0
	origin := &models.Community{
		ID:        "origin-1",
		Name:      "Sudbury",
		Latitude:  &originLat,
		Longitude: &originLon,
	}

	destLat, destLon := 48.47, -81.33
	dest := &models.CommunityWithDistance{
		Community: models.Community{
			ID:        "dest-1",
			Name:      "Timmins",
			Latitude:  &destLat,
			Longitude: &destLon,
		},
		DistanceKm: 220,
	}

	cwt, computeErr := svc.computeSingle(context.Background(), origin, dest, "driving")
	require.NoError(t, computeErr)
	require.NotNil(t, cwt)
	assert.Equal(t, "dest-1", cwt.CommunityID)
	assert.Equal(t, "Timmins", cwt.Name)
	assert.InDelta(t, 30.0, cwt.DurationMinutes, 0.001)
	assert.InDelta(t, 30.0, cwt.DistanceKm, 0.001)
	assert.Equal(t, "driving", cwt.TransportMode)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestComputeSingle_NoCoordsOnDest(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	log := testLogger(t)
	travelRepo := repository.NewTravelTimeRepository(db, log)
	svc := NewTravelTimeService(nil, travelRepo, nil, log)

	// Return no cached entry
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, origin_community_id")).
		WithArgs("origin-1", "dest-no-coords", "driving").
		WillReturnRows(sqlmock.NewRows(nil))

	originLat, originLon := 46.49, -81.0
	origin := &models.Community{
		ID:        "origin-1",
		Latitude:  &originLat,
		Longitude: &originLon,
	}

	dest := &models.CommunityWithDistance{
		Community: models.Community{
			ID:        "dest-no-coords",
			Name:      "No Coords",
			Latitude:  nil,
			Longitude: nil,
		},
	}

	cwt, computeErr := svc.computeSingle(context.Background(), origin, dest, "driving")
	require.Error(t, computeErr)
	assert.Nil(t, cwt)
	assert.Contains(t, computeErr.Error(), "no coordinates")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestComputeMatrix_OriginNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	log := testLogger(t)
	communityRepo := repository.NewCommunityRepository(db, log)
	svc := NewTravelTimeService(nil, nil, communityRepo, log)

	// Return empty for GetByID
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, slug")).
		WithArgs("nonexistent").
		WillReturnRows(sqlmock.NewRows(nil))

	_, computeErr := svc.ComputeMatrix(context.Background(), "nonexistent", 100, 60, "driving")
	require.Error(t, computeErr)
	assert.Contains(t, computeErr.Error(), "not found")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestComputeMatrix_OriginNoCoords(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	log := testLogger(t)
	communityRepo := repository.NewCommunityRepository(db, log)
	svc := NewTravelTimeService(nil, nil, communityRepo, log)

	now := time.Now()
	cols := []string{
		"id", "name", "slug", "community_type", "province", "region",
		"inac_id", "statcan_csd", "osm_relation_id", "wikidata_qid", "latitude", "longitude",
		"nation", "treaty", "language_group", "reserve_name", "population", "population_year",
		"website", "feed_url", "data_source", "source_id", "enabled", "created_at", "updated_at",
	}

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, slug")).
		WithArgs("no-coords").
		WillReturnRows(
			sqlmock.NewRows(cols).AddRow(
				"no-coords", "No Coords", "no-coords", "city", nil, nil,
				nil, nil, nil, nil, nil, nil,
				nil, nil, nil, nil, nil, nil,
				nil, nil, "manual", nil, true, now, now,
			),
		)

	_, computeErr := svc.ComputeMatrix(context.Background(), "no-coords", 100, 60, "driving")
	require.Error(t, computeErr)
	assert.Contains(t, computeErr.Error(), "no coordinates")

	require.NoError(t, mock.ExpectationsWereMet())
}
