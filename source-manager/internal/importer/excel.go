package importer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/xuri/excelize/v2"
)

// Header names for flexible column mapping (case-insensitive).
// Supports both the original format and common spreadsheet formats.
const (
	headerRowIndex = 1 // Excel rows are 1-based, header is row 1
)

// columnMap stores the index for each recognized header.
type columnMap struct {
	name      int
	url       int
	enabled   int
	rateLimit int
	maxDepth  int
	time      int
	selectors int
}

// headerAliases maps various header names to their canonical field.
var headerAliases = map[string]string{
	// Name field aliases
	"name":           "name",
	"news site name": "name",
	"site name":      "name",
	"source name":    "name",
	"source":         "name",
	"title":          "name",
	"website name":   "name",
	// URL field aliases
	"url":      "url",
	"website":  "url",
	"site url": "url",
	"link":     "url",
	// Enabled field aliases
	"enabled": "enabled",
	"status":  "enabled",
	"active":  "enabled",
	// RateLimit field aliases
	"rate_limit": "ratelimit",
	"ratelimit":  "ratelimit",
	"rate limit": "ratelimit",
	// MaxDepth field aliases
	"max_depth": "maxdepth",
	"maxdepth":  "maxdepth",
	"max depth": "maxdepth",
	"depth":     "maxdepth",
	// Time field aliases
	"time":  "time",
	"times": "time",
	// Selectors field aliases
	"selectors": "selectors",
	"selector":  "selectors",
}

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

// parseTimeJSON parses a JSON array of strings. Empty input returns nil, nil.
func parseTimeJSON(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// parseSelectorsJSON parses a JSON object into SelectorConfig. Empty input returns zero value, nil.
func parseSelectorsJSON(s string) (models.SelectorConfig, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return models.SelectorConfig{}, nil
	}
	var out models.SelectorConfig
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return models.SelectorConfig{}, err
	}
	return out, nil
}

// validateRequiredColumns returns an ImportError if name or url column is missing; Row is 1 (header row).
func validateRequiredColumns(colMap columnMap) *ImportError {
	if colMap.name == -1 && colMap.url == -1 {
		return &ImportError{Row: 1, Error: "missing required columns: need 'Name' (or 'News Site Name') and 'URL' headers"}
	}
	if colMap.name == -1 {
		return &ImportError{Row: 1, Error: "missing required column: 'Name' (or 'News Site Name', 'Site Name', 'Source')"}
	}
	if colMap.url == -1 {
		return &ImportError{Row: 1, Error: "missing required column: 'URL' (or 'Website', 'Link')"}
	}
	return nil
}

// openExcelRows opens the workbook from reader, reads the first sheet, and returns all rows.
// Returns an error on open/sheet/read failure; returns [][]string{}, nil when the sheet has no rows.
func openExcelRows(reader io.Reader) ([][]string, error) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, errors.New("no sheets found in Excel file")
	}

	excelRows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read rows: %w", err)
	}

	if excelRows == nil {
		return [][]string{}, nil
	}
	return excelRows, nil
}

// parseHeaders builds a column map from the header row.
func parseHeaders(headerRow []string) columnMap {
	colMap := columnMap{
		name:      -1,
		url:       -1,
		enabled:   -1,
		rateLimit: -1,
		maxDepth:  -1,
		time:      -1,
		selectors: -1,
	}

	for i, header := range headerRow {
		normalized := strings.ToLower(strings.TrimSpace(header))
		if field, ok := headerAliases[normalized]; ok {
			switch field {
			case "name":
				colMap.name = i
			case "url":
				colMap.url = i
			case "enabled":
				colMap.enabled = i
			case "ratelimit":
				colMap.rateLimit = i
			case "maxdepth":
				colMap.maxDepth = i
			case "time":
				colMap.time = i
			case "selectors":
				colMap.selectors = i
			}
		}
	}

	return colMap
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

	// Need at least a header row
	if len(excelRows) == 0 {
		return []SourceRow{}, []ImportError{}
	}

	// Parse headers to build column map
	colMap := parseHeaders(excelRows[0])

	// Validate required columns exist
	if colMap.name == -1 && colMap.url == -1 {
		return nil, []ImportError{{Row: 1, Error: "missing required columns: need 'Name' (or 'News Site Name') and 'URL' headers"}}
	}
	if colMap.name == -1 {
		return nil, []ImportError{{Row: 1, Error: "missing required column: 'Name' (or 'News Site Name', 'Site Name', 'Source')"}}
	}
	if colMap.url == -1 {
		return nil, []ImportError{{Row: 1, Error: "missing required column: 'URL' (or 'Website', 'Link')"}}
	}

	// Skip header row, check if there's any data
	if len(excelRows) <= headerRowIndex {
		return []SourceRow{}, []ImportError{}
	}

	var rows []SourceRow
	var errs []ImportError

	// Parse data rows (skip header at index 0)
	for i := headerRowIndex; i < len(excelRows); i++ {
		cells := excelRows[i]
		rowNum := i + 1 // Excel rows are 1-based

		// Skip empty rows
		if isEmptyRow(cells) {
			continue
		}

		// Parse the row using column map
		sourceRow := parseRowWithMap(cells, rowNum, colMap)

		// Skip rows without URL (don't treat as error)
		if strings.TrimSpace(sourceRow.URL) == "" {
			continue
		}

		// Validate the row
		if errMsg := ValidateRow(sourceRow); errMsg != "" {
			errs = append(errs, ImportError{Row: rowNum, Error: errMsg})
			continue
		}

		rows = append(rows, sourceRow)
	}

	// If any validation errors, return nil rows with all errors
	if len(errs) > 0 {
		return nil, errs
	}

	return rows, errs
}

// parseRowWithMap converts Excel row cells to a SourceRow using the column map.
func parseRowWithMap(cells []string, rowNum int, colMap columnMap) SourceRow {
	row := SourceRow{Row: rowNum}

	// Helper to safely get cell value
	getCell := func(idx int) string {
		if idx >= 0 && idx < len(cells) {
			return strings.TrimSpace(cells[idx])
		}
		return ""
	}

	row.Name = getCell(colMap.name)
	row.URL = getCell(colMap.url)

	// Parse enabled - support "Active", "true", "yes", "1", etc.
	if colMap.enabled >= 0 {
		enabledStr := strings.ToLower(getCell(colMap.enabled))
		row.Enabled = enabledStr == "active" || enabledStr == "true" || enabledStr == "yes" || enabledStr == "y" || enabledStr == "1"
	} else {
		row.Enabled = true // Default to enabled if no status column
	}

	row.RateLimit = getCell(colMap.rateLimit)
	row.MaxDepth = parseInt(getCell(colMap.maxDepth))
	row.Time = getCell(colMap.time)
	row.Selectors = getCell(colMap.selectors)

	return row
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
	if _, err := parseTimeJSON(row.Time); err != nil {
		return "time must be a valid JSON array"
	}

	// Selectors must be valid JSON object if provided
	if _, err := parseSelectorsJSON(row.Selectors); err != nil {
		return "selectors must be valid JSON"
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
		RateLimit: models.NormalizeRateLimit(row.RateLimit),
		MaxDepth:  row.MaxDepth,
	}

	// Parse Time JSON array
	timeArr, err := parseTimeJSON(row.Time)
	if err != nil {
		return nil, fmt.Errorf("parse time: %w", err)
	}
	if timeArr != nil {
		source.Time = models.StringArray(timeArr)
	}

	// Parse Selectors JSON object
	selectors, err := parseSelectorsJSON(row.Selectors)
	if err != nil {
		return nil, fmt.Errorf("parse selectors: %w", err)
	}
	source.Selectors = selectors

	return source, nil
}
