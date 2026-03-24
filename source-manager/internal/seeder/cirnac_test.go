//nolint:testpackage // Testing unexported CSV parsing helpers requires same-package access
package seeder

import (
	"bytes"
	"context"
	"encoding/csv"
	"testing"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
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

func TestBuildColumnIndex(t *testing.T) {
	t.Parallel()

	header := []string{"BAND_NUMBER", "BAND_NAME", "LATITUDE", "LONGITUDE"}
	idx := buildColumnIndex(header)

	assert.Equal(t, 0, idx["BAND_NUMBER"])
	assert.Equal(t, 1, idx["BAND_NAME"])
	assert.Equal(t, 2, idx["LATITUDE"])
	assert.Equal(t, 3, idx["LONGITUDE"])
}

func TestBuildColumnIndex_WithSpaces(t *testing.T) {
	t.Parallel()

	header := []string{" BAND_NUMBER ", "BAND_NAME"}
	idx := buildColumnIndex(header)

	assert.Equal(t, 0, idx["BAND_NUMBER"])
	assert.Equal(t, 1, idx["BAND_NAME"])
}

func TestValidateColumns_AllPresent(t *testing.T) {
	t.Parallel()

	colIndex := map[string]int{
		"BAND_NUMBER": 0,
		"BAND_NAME":   1,
		"LATITUDE":    2,
		"LONGITUDE":   3,
	}
	err := validateColumns(colIndex)
	assert.NoError(t, err)
}

func TestValidateColumns_MissingColumn(t *testing.T) {
	t.Parallel()

	colIndex := map[string]int{
		"BAND_NUMBER": 0,
		"BAND_NAME":   1,
		// LATITUDE and LONGITUDE missing
	}
	err := validateColumns(colIndex)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required column")
}

func TestParseCoords_ValidCoords(t *testing.T) {
	t.Parallel()

	record := []string{"123", "Test Band", "46.49", "-81.00"}
	colIndex := map[string]int{
		"LATITUDE":  2,
		"LONGITUDE": 3,
	}

	lat, lng := parseCoords(record, colIndex)
	require.NotNil(t, lat)
	require.NotNil(t, lng)
	assert.InDelta(t, 46.49, *lat, 0.001)
	assert.InDelta(t, -81.00, *lng, 0.001)
}

func TestParseCoords_EmptyStrings(t *testing.T) {
	t.Parallel()

	record := []string{"123", "Test Band", "", ""}
	colIndex := map[string]int{
		"LATITUDE":  2,
		"LONGITUDE": 3,
	}

	lat, lng := parseCoords(record, colIndex)
	assert.Nil(t, lat)
	assert.Nil(t, lng)
}

func TestParseCoords_InvalidValues(t *testing.T) {
	t.Parallel()

	record := []string{"123", "Test Band", "not-a-number", "-81.00"}
	colIndex := map[string]int{
		"LATITUDE":  2,
		"LONGITUDE": 3,
	}

	lat, lng := parseCoords(record, colIndex)
	assert.Nil(t, lat)
	assert.Nil(t, lng)
}

func TestBuildCommunity(t *testing.T) {
	t.Parallel()

	lat := 46.49
	lng := -81.00
	c := buildCommunity("123", "Test First Nation", "ON", &lat, &lng)

	assert.Equal(t, "Test First Nation", c.Name)
	assert.Equal(t, "test-first-nation", c.Slug)
	assert.Equal(t, communityTypeFN, c.CommunityType)
	assert.Equal(t, "123", *c.InacID)
	assert.Equal(t, "ON", *c.Province)
	assert.InDelta(t, 46.49, *c.Latitude, 0.001)
	assert.InDelta(t, -81.00, *c.Longitude, 0.001)
	assert.Equal(t, dataSourceCIRNAC, c.DataSource)
	assert.True(t, c.Enabled)
}

func TestBuildCommunity_EmptyProvince(t *testing.T) {
	t.Parallel()

	c := buildCommunity("456", "Band Name", "", nil, nil)

	assert.Equal(t, "Band Name", c.Name)
	assert.Nil(t, c.Province)
	assert.Nil(t, c.Latitude)
	assert.Nil(t, c.Longitude)
}

func TestSeedFromReader_ValidCSV(t *testing.T) {
	t.Parallel()

	csvData := "BAND_NUMBER,BAND_NAME,LATITUDE,LONGITUDE\n" +
		"100,Test First Nation,46.49,-81.00\n" +
		"200,Second Band,48.47,-81.33\n"

	reader := csv.NewReader(bytes.NewReader([]byte(csvData)))

	seeder := &CIRNACSeeder{
		repo:   nil,
		logger: testLogger(t),
	}

	result, err := seeder.seedFromReader(context.Background(), reader, true) // dryRun=true
	require.NoError(t, err)
	assert.Equal(t, 2, result.Total)
	assert.Equal(t, 2, result.WouldCreate)
	assert.Equal(t, 0, result.Errors)
	assert.Equal(t, 0, result.Skipped)
}

func TestSeedFromReader_SkipsMissingBandNumber(t *testing.T) {
	t.Parallel()

	csvData := "BAND_NUMBER,BAND_NAME,LATITUDE,LONGITUDE\n" +
		",Missing Number,46.49,-81.00\n"

	reader := csv.NewReader(bytes.NewReader([]byte(csvData)))

	seeder := &CIRNACSeeder{
		repo:   nil,
		logger: testLogger(t),
	}

	result, err := seeder.seedFromReader(context.Background(), reader, true)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, 0, result.WouldCreate)
	assert.Equal(t, 1, result.Skipped)
}

func TestSeedFromReader_SkipsMissingBandName(t *testing.T) {
	t.Parallel()

	csvData := "BAND_NUMBER,BAND_NAME,LATITUDE,LONGITUDE\n" +
		"100,,46.49,-81.00\n"

	reader := csv.NewReader(bytes.NewReader([]byte(csvData)))

	seeder := &CIRNACSeeder{
		repo:   nil,
		logger: testLogger(t),
	}

	result, err := seeder.seedFromReader(context.Background(), reader, true)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, 0, result.WouldCreate)
	assert.Equal(t, 1, result.Skipped)
}

func TestSeedFromReader_MissingRequiredColumn(t *testing.T) {
	t.Parallel()

	csvData := "BAND_NUMBER,MISSING_COLUMN\n100,data\n"

	reader := csv.NewReader(bytes.NewReader([]byte(csvData)))

	seeder := &CIRNACSeeder{
		repo:   nil,
		logger: testLogger(t),
	}

	_, err := seeder.seedFromReader(context.Background(), reader, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required column")
}

func TestNewCIRNACSeeder(t *testing.T) {
	t.Parallel()

	seeder := NewCIRNACSeeder(nil, nil)
	assert.NotNil(t, seeder)
}
