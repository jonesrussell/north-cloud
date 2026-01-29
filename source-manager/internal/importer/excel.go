package importer

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/xuri/excelize/v2"
)

// Column indices for Excel spreadsheet (0-based).
const (
	colName      = 0 // Column A
	colURL       = 1 // Column B
	colEnabled   = 2 // Column C
	colRateLimit = 3 // Column D
	colMaxDepth  = 4 // Column E
	colTime      = 5 // Column F
	colSelectors = 6 // Column G

	minRequiredColumns = 7
	headerRowIndex     = 1 // Excel rows are 1-based, header is row 1
)

// SourceRow represents a parsed row from the Excel spreadsheet.
type SourceRow struct {
	Row       int // Excel row number (for error reporting)
	Name      string
	URL       string
	Enabled   bool
	RateLimit string
	MaxDepth  int
	Time      string // Raw JSON string
	Selectors string // Raw JSON string
}

// ImportError represents a validation error for a specific row.
type ImportError struct {
	Row   int    `json:"row"`
	Error string `json:"error"`
}

// ParseExcelFile parses an Excel file from an io.Reader and returns parsed rows and any validation errors.
// If any validation errors occur, returns nil rows with all errors.
func ParseExcelFile(reader io.Reader) ([]SourceRow, []ImportError) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, []ImportError{{Row: 0, Error: "failed to open Excel file: " + err.Error()}}
	}
	defer f.Close()

	// Get the first sheet
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, []ImportError{{Row: 0, Error: "no sheets found in Excel file"}}
	}

	// Get all rows
	excelRows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, []ImportError{{Row: 0, Error: "failed to read rows: " + err.Error()}}
	}

	// Skip header row, check if there's any data
	if len(excelRows) <= headerRowIndex {
		return []SourceRow{}, []ImportError{}
	}

	var rows []SourceRow
	var errors []ImportError

	// Parse data rows (skip header at index 0)
	for i := headerRowIndex; i < len(excelRows); i++ {
		cells := excelRows[i]
		rowNum := i + 1 // Excel rows are 1-based

		// Skip empty rows
		if isEmptyRow(cells) {
			continue
		}

		// Parse the row
		sourceRow := parseRow(cells, rowNum)

		// Validate the row
		if errMsg := ValidateRow(sourceRow); errMsg != "" {
			errors = append(errors, ImportError{Row: rowNum, Error: errMsg})
			continue
		}

		rows = append(rows, sourceRow)
	}

	// If any validation errors, return nil rows with all errors
	if len(errors) > 0 {
		return nil, errors
	}

	return rows, errors
}

// parseRow converts Excel row cells to a SourceRow.
func parseRow(cells []string, rowNum int) SourceRow {
	row := SourceRow{Row: rowNum}

	if len(cells) > colName {
		row.Name = strings.TrimSpace(cells[colName])
	}
	if len(cells) > colURL {
		row.URL = strings.TrimSpace(cells[colURL])
	}
	if len(cells) > colEnabled {
		row.Enabled = parseBool(cells[colEnabled])
	}
	if len(cells) > colRateLimit {
		row.RateLimit = strings.TrimSpace(cells[colRateLimit])
	}
	if len(cells) > colMaxDepth {
		row.MaxDepth = parseInt(cells[colMaxDepth])
	}
	if len(cells) > colTime {
		row.Time = strings.TrimSpace(cells[colTime])
	}
	if len(cells) > colSelectors {
		row.Selectors = strings.TrimSpace(cells[colSelectors])
	}

	return row
}

// parseBool parses a string to bool, treating "true", "1", "yes", "y" as true.
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes" || s == "y"
}

// parseInt parses a string to int, returns 0 on error.
func parseInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return val
}

// isEmptyRow checks if all cells in a row are empty.
func isEmptyRow(cells []string) bool {
	for _, cell := range cells {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// ValidateRow validates a single row and returns an error message or empty string.
func ValidateRow(row SourceRow) string {
	// Required fields
	if strings.TrimSpace(row.Name) == "" {
		return "name is required"
	}
	if strings.TrimSpace(row.URL) == "" {
		return "url is required"
	}

	// URL must be http or https
	if !strings.HasPrefix(row.URL, "http://") && !strings.HasPrefix(row.URL, "https://") {
		return "url must start with http:// or https://"
	}

	// max_depth must be non-negative
	if row.MaxDepth < 0 {
		return "max_depth must be non-negative"
	}

	// Time must be valid JSON array if provided
	if row.Time != "" {
		var timeArr []string
		if err := json.Unmarshal([]byte(row.Time), &timeArr); err != nil {
			return "time must be a valid JSON array"
		}
	}

	// Selectors must be valid JSON object if provided
	if row.Selectors != "" {
		var selectors map[string]any
		if err := json.Unmarshal([]byte(row.Selectors), &selectors); err != nil {
			return "selectors must be valid JSON"
		}
	}

	return ""
}

// ToSource converts a validated SourceRow to a models.Source.
// This should only be called after validation passes.
func ToSource(row SourceRow) (*models.Source, error) {
	source := &models.Source{
		Name:      row.Name,
		URL:       row.URL,
		Enabled:   row.Enabled,
		RateLimit: row.RateLimit,
		MaxDepth:  row.MaxDepth,
	}

	// Parse Time JSON array
	if row.Time != "" {
		var timeArr []string
		if err := json.Unmarshal([]byte(row.Time), &timeArr); err != nil {
			return nil, fmt.Errorf("parse time: %w", err)
		}
		source.Time = models.StringArray(timeArr)
	}

	// Parse Selectors JSON object
	if row.Selectors != "" {
		var selectors models.SelectorConfig
		if err := json.Unmarshal([]byte(row.Selectors), &selectors); err != nil {
			return nil, fmt.Errorf("parse selectors: %w", err)
		}
		source.Selectors = selectors
	}

	return source, nil
}
