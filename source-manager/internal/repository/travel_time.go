package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// TravelTimeEntry represents a cached travel time between two communities.
type TravelTimeEntry struct {
	ID                     int       `db:"id"                       json:"id"`
	OriginCommunityID      string    `db:"origin_community_id"      json:"origin_community_id"`
	DestinationCommunityID string    `db:"destination_community_id" json:"destination_community_id"`
	TransportMode          string    `db:"transport_mode"           json:"transport_mode"`
	DurationSeconds        int       `db:"duration_seconds"         json:"duration_seconds"`
	DistanceMeters         int       `db:"distance_meters"          json:"distance_meters"`
	ComputedAt             time.Time `db:"computed_at"              json:"computed_at"`
}

// CommunityWithTravelTime extends community info with travel time data.
type CommunityWithTravelTime struct {
	CommunityID     string  `json:"id"`
	Name            string  `json:"name"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	DurationMinutes float64 `json:"duration_minutes"`
	DistanceKm      float64 `json:"distance_km"`
	TransportMode   string  `json:"transport_mode"`
}

// TravelTimeRepository provides CRUD operations for the travel_time_cache table.
type TravelTimeRepository struct {
	db     *sql.DB
	logger infralogger.Logger
}

// NewTravelTimeRepository creates a new TravelTimeRepository.
func NewTravelTimeRepository(db *sql.DB, log infralogger.Logger) *TravelTimeRepository {
	return &TravelTimeRepository{
		db:     db,
		logger: log,
	}
}

// GetCachedTravelTime returns a cached travel time entry, or nil if not found.
func (r *TravelTimeRepository) GetCachedTravelTime(
	ctx context.Context, originID, destID, mode string,
) (*TravelTimeEntry, error) {
	query := `SELECT id, origin_community_id, destination_community_id,
		transport_mode, duration_seconds, distance_meters, computed_at
		FROM travel_time_cache
		WHERE origin_community_id = $1
			AND destination_community_id = $2
			AND transport_mode = $3`

	var entry TravelTimeEntry
	err := r.db.QueryRowContext(ctx, query, originID, destID, mode).Scan(
		&entry.ID, &entry.OriginCommunityID, &entry.DestinationCommunityID,
		&entry.TransportMode, &entry.DurationSeconds, &entry.DistanceMeters,
		&entry.ComputedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil //nolint:nilnil // nil,nil = "not found" per interface contract
		}
		return nil, fmt.Errorf("get cached travel time: %w", err)
	}

	return &entry, nil
}

// UpsertTravelTime inserts or updates a travel time cache entry.
func (r *TravelTimeRepository) UpsertTravelTime(ctx context.Context, entry TravelTimeEntry) error {
	query := `INSERT INTO travel_time_cache (
			origin_community_id, destination_community_id, transport_mode,
			duration_seconds, distance_meters, computed_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (origin_community_id, destination_community_id, transport_mode)
		DO UPDATE SET
			duration_seconds = EXCLUDED.duration_seconds,
			distance_meters = EXCLUDED.distance_meters,
			computed_at = EXCLUDED.computed_at`

	_, err := r.db.ExecContext(ctx, query,
		entry.OriginCommunityID, entry.DestinationCommunityID, entry.TransportMode,
		entry.DurationSeconds, entry.DistanceMeters, entry.ComputedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert travel time: %w", err)
	}

	return nil
}

const (
	secondsPerMinute = 60
)

// GetMatrix returns all cached travel times from a given origin within maxMinutes.
func (r *TravelTimeRepository) GetMatrix(
	ctx context.Context, originID string, maxMinutes int, mode string,
) ([]CommunityWithTravelTime, error) {
	maxSeconds := maxMinutes * secondsPerMinute

	query := `SELECT c.id, c.name, c.latitude, c.longitude,
			t.duration_seconds, t.distance_meters, t.transport_mode
		FROM travel_time_cache t
		JOIN communities c ON c.id = t.destination_community_id
		WHERE t.origin_community_id = $1
			AND t.transport_mode = $2
			AND t.duration_seconds <= $3
		ORDER BY t.duration_seconds ASC`

	rows, err := r.db.QueryContext(ctx, query, originID, mode, maxSeconds)
	if err != nil {
		return nil, fmt.Errorf("get travel time matrix: %w", err)
	}
	defer rows.Close()

	var results []CommunityWithTravelTime
	for rows.Next() {
		var cwt CommunityWithTravelTime
		var durationSecs int
		var distanceMeters int
		scanErr := rows.Scan(
			&cwt.CommunityID, &cwt.Name, &cwt.Latitude, &cwt.Longitude,
			&durationSecs, &distanceMeters, &cwt.TransportMode,
		)
		if scanErr != nil {
			return nil, fmt.Errorf("scan travel time row: %w", scanErr)
		}
		cwt.DurationMinutes = float64(durationSecs) / float64(secondsPerMinute)
		cwt.DistanceKm = float64(distanceMeters) / metersPerKm
		results = append(results, cwt)
	}

	if closeErr := rows.Err(); closeErr != nil {
		return nil, fmt.Errorf("travel time matrix rows: %w", closeErr)
	}

	return results, nil
}

const metersPerKm = 1000.0
