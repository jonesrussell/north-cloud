package seeder

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
)

const (
	dataSourceCIRNAC   = "cirnac"
	communityTypeFN    = "first_nation"
	colBandNumber      = "BAND_NUMBER"
	colBandName        = "BAND_NAME"
	colLatitude        = "LATITUDE"
	colLongitude       = "LONGITUDE"
	expectedMinColumns = 4
)

// CIRNACResult holds the result of a CIRNAC seed operation.
type CIRNACResult struct {
	Total       int
	Created     int
	Updated     int
	WouldCreate int
	Skipped     int
	Errors      int
}

// CIRNACSeeder imports CIRNAC open data CSV into the communities table.
type CIRNACSeeder struct {
	repo   *repository.CommunityRepository
	logger infralogger.Logger
}

// NewCIRNACSeeder creates a new seeder.
func NewCIRNACSeeder(repo *repository.CommunityRepository, log infralogger.Logger) *CIRNACSeeder {
	return &CIRNACSeeder{repo: repo, logger: log}
}

// SeedFromFile reads a CIRNAC CSV file and upserts communities.
func (s *CIRNACSeeder) SeedFromFile(ctx context.Context, csvPath string, dryRun bool) (*CIRNACResult, error) {
	data, err := os.ReadFile(csvPath)
	if err != nil {
		return nil, fmt.Errorf("read CSV file: %w", err)
	}

	// Strip UTF-8 BOM if present
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	reader := csv.NewReader(bytes.NewReader(data))

	return s.seedFromReader(ctx, reader, dryRun)
}

// seedFromReader processes CSV records from a reader.
func (s *CIRNACSeeder) seedFromReader(ctx context.Context, reader *csv.Reader, dryRun bool) (*CIRNACResult, error) {
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}

	colIndex := buildColumnIndex(header)
	if validateErr := validateColumns(colIndex); validateErr != nil {
		return nil, validateErr
	}

	result := &CIRNACResult{}

	for {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			result.Errors++
			s.logger.Error("Failed to read CSV row", infralogger.Error(readErr))
			continue
		}

		result.Total++
		s.processRecord(ctx, record, colIndex, dryRun, result)
	}

	return result, nil
}

// processRecord converts a CSV row to a Community and upserts it.
func (s *CIRNACSeeder) processRecord(
	ctx context.Context,
	record []string,
	colIndex map[string]int,
	dryRun bool,
	result *CIRNACResult,
) {
	bandNumber := strings.TrimSpace(record[colIndex[colBandNumber]])
	bandName := strings.TrimSpace(record[colIndex[colBandName]])

	if bandNumber == "" || bandName == "" {
		result.Skipped++
		return
	}

	lat, lng := parseCoords(record, colIndex)
	province := ""
	if lat != nil && lng != nil {
		province = InferProvince(*lat, *lng)
	}

	community := buildCommunity(bandNumber, bandName, province, lat, lng)

	if dryRun {
		s.logger.Info("Would upsert community",
			infralogger.String("inac_id", bandNumber),
			infralogger.String("name", bandName),
			infralogger.String("province", province),
		)
		result.WouldCreate++
		return
	}

	if err := s.repo.UpsertByInacID(ctx, &community); err != nil {
		result.Errors++
		s.logger.Error("Failed to upsert community",
			infralogger.String("inac_id", bandNumber),
			infralogger.Error(err),
		)
		return
	}

	result.Created++
}

// buildCommunity constructs a Community model from parsed CSV fields.
func buildCommunity(bandNumber, bandName, province string, lat, lng *float64) models.Community {
	c := models.Community{
		Name:          bandName,
		Slug:          Slugify(bandName),
		CommunityType: communityTypeFN,
		InacID:        &bandNumber,
		Latitude:      lat,
		Longitude:     lng,
		DataSource:    dataSourceCIRNAC,
		Enabled:       true,
	}
	if province != "" {
		c.Province = &province
	}
	return c
}

// parseCoords extracts lat/lng from a CSV record. Returns nil for missing/invalid values.
func parseCoords(record []string, colIndex map[string]int) (latPtr, lngPtr *float64) {
	latStr := strings.TrimSpace(record[colIndex[colLatitude]])
	lngStr := strings.TrimSpace(record[colIndex[colLongitude]])

	if latStr == "" || lngStr == "" {
		return nil, nil
	}

	latVal, latErr := strconv.ParseFloat(latStr, 64)
	lngVal, lngErr := strconv.ParseFloat(lngStr, 64)
	if latErr != nil || lngErr != nil {
		return nil, nil
	}

	return &latVal, &lngVal
}

// buildColumnIndex maps header names to their indices.
func buildColumnIndex(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, col := range header {
		idx[strings.TrimSpace(col)] = i
	}
	return idx
}

// validateColumns ensures all required columns exist.
func validateColumns(colIndex map[string]int) error {
	required := []string{colBandNumber, colBandName, colLatitude, colLongitude}
	for _, col := range required {
		if _, ok := colIndex[col]; !ok {
			return fmt.Errorf("missing required column: %s", col)
		}
	}
	return nil
}
