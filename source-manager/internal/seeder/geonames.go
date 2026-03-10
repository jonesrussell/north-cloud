package seeder

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
)

const (
	dataSourceStatscan = "statscan"

	// GeoNames column indices (tab-separated, fixed layout).
	gnColID          = 0
	gnColName        = 1
	gnColLatitude    = 4
	gnColLongitude   = 5
	gnColFeatureCode = 7
	gnColAdmin1      = 10
	gnColPopulation  = 14
	gnColFieldCount  = 19 // minimum expected columns

	// Population thresholds for community type inference.
	cityPopulationMin      = 100000
	townPopulationMin      = 1000
	geoNamesPopulationYear = 2021
)

// populatedPlaceFeatureCodes filters GeoNames records to populated places.
//
//nolint:gochecknoglobals // static lookup set
var populatedPlaceFeatureCodes = map[string]bool{
	"PPL":   true, // populated place
	"PPLA":  true, // seat of first-order admin division
	"PPLA2": true, // seat of second-order admin division
	"PPLC":  true, // capital of a political entity
	"PPLA3": true, // seat of third-order admin division
	"PPLA4": true, // seat of fourth-order admin division
	"PPLX":  true, // section of populated place
}

// admin1ToProvince maps GeoNames admin1 codes to Canadian province abbreviations.
//
//nolint:gochecknoglobals // static lookup table
var admin1ToProvince = map[string]string{
	"01": "AB", "02": "BC", "03": "MB", "04": "NB",
	"05": "NL", "07": "NS", "08": "ON", "09": "PE",
	"10": "QC", "11": "SK", "12": "YT", "13": "NT",
	"14": "NU",
}

// GeoNamesResult holds the result of a GeoNames seed operation.
type GeoNamesResult struct {
	Total       int
	Created     int
	Updated     int
	WouldCreate int
	Skipped     int
	Errors      int
}

// GeoNamesSeeder imports GeoNames populated places into the communities table.
type GeoNamesSeeder struct {
	repo           *repository.CommunityRepository
	logger         infralogger.Logger
	provinceFilter string // e.g. "ON" for Ontario-only
}

// NewGeoNamesSeeder creates a new seeder. Set provinceFilter to "" for all provinces.
func NewGeoNamesSeeder(
	repo *repository.CommunityRepository,
	log infralogger.Logger,
	provinceFilter string,
) *GeoNamesSeeder {
	return &GeoNamesSeeder{
		repo:           repo,
		logger:         log,
		provinceFilter: strings.ToUpper(provinceFilter),
	}
}

// SeedFromFile reads a GeoNames tab-separated file and upserts communities.
func (s *GeoNamesSeeder) SeedFromFile(ctx context.Context, filePath string, dryRun bool) (*GeoNamesResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open GeoNames file: %w", err)
	}
	defer file.Close()

	result := &GeoNamesResult{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < gnColFieldCount {
			result.Skipped++
			continue
		}

		s.processGeoNamesRecord(ctx, fields, dryRun, result)
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return result, fmt.Errorf("scan GeoNames file: %w", scanErr)
	}

	return result, nil
}

// processGeoNamesRecord handles a single GeoNames tab-separated line.
func (s *GeoNamesSeeder) processGeoNamesRecord(
	ctx context.Context,
	fields []string,
	dryRun bool,
	result *GeoNamesResult,
) {
	featureCode := fields[gnColFeatureCode]
	if !populatedPlaceFeatureCodes[featureCode] {
		result.Skipped++
		return
	}

	admin1 := fields[gnColAdmin1]
	province := admin1ToProvince[admin1]
	if province == "" {
		result.Skipped++
		return
	}

	if s.provinceFilter != "" && province != s.provinceFilter {
		result.Skipped++
		return
	}

	result.Total++

	community := s.buildGeoNamesCommunity(fields, province)

	if dryRun {
		s.logger.Info("Would upsert municipality",
			infralogger.String("name", community.Name),
			infralogger.String("province", province),
			infralogger.String("type", community.CommunityType),
		)
		result.WouldCreate++
		return
	}

	if err := s.repo.UpsertByStatCanCSD(ctx, &community); err != nil {
		result.Errors++
		s.logger.Error("Failed to upsert municipality",
			infralogger.String("name", community.Name),
			infralogger.Error(err),
		)
		return
	}

	result.Created++
}

// buildGeoNamesCommunity constructs a Community from GeoNames fields.
func (s *GeoNamesSeeder) buildGeoNamesCommunity(fields []string, province string) models.Community {
	geoID := fields[gnColID]
	name := fields[gnColName]
	pop, _ := strconv.Atoi(fields[gnColPopulation])

	communityType := InferCommunityType(pop)
	popYear := geoNamesPopulationYear

	c := models.Community{
		Name:          name,
		Slug:          Slugify(name),
		CommunityType: communityType,
		Province:      &province,
		StatCanCSD:    &geoID,
		DataSource:    dataSourceStatscan,
		Enabled:       true,
	}

	lat, lng := parseGeoNamesCoords(fields)
	c.Latitude = lat
	c.Longitude = lng

	if pop > 0 {
		c.Population = &pop
		c.PopulationYear = &popYear
	}

	return c
}

// parseGeoNamesCoords extracts lat/lng from GeoNames fields. Returns nil for invalid values.
func parseGeoNamesCoords(fields []string) (latPtr, lngPtr *float64) {
	latVal, latErr := strconv.ParseFloat(fields[gnColLatitude], 64)
	lngVal, lngErr := strconv.ParseFloat(fields[gnColLongitude], 64)
	if latErr != nil || lngErr != nil {
		return nil, nil
	}
	return &latVal, &lngVal
}

// InferCommunityType returns a community type based on population.
func InferCommunityType(population int) string {
	switch {
	case population >= cityPopulationMin:
		return "city"
	case population >= townPopulationMin:
		return "town"
	default:
		return "settlement"
	}
}
