package importer

import (
	"encoding/json"
	"strings"
)

// Column indices for Excel spreadsheet (0-based).
// These constants are used by ParseExcelFile (implemented in Task 2.3).
//
//nolint:unused // Constants will be used by ParseExcelFile in Task 2.3
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
