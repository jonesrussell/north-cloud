package importer

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
