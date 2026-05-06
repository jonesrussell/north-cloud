package severity

import (
	"strings"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

// Table maps lowercase substance names to their known severity levels.
type Table map[string]domain.Severity

// NewTable constructs a Table from raw input, normalising every key to
// lowercase with surrounding whitespace trimmed.
func NewTable(raw map[string]domain.Severity) Table {
	t := make(Table, len(raw))
	for k, v := range raw {
		t[strings.ToLower(strings.TrimSpace(k))] = v
	}

	return t
}

// Lookup returns the Severity for the given substance name.
// The lookup is case-insensitive and whitespace-tolerant.
func (t Table) Lookup(substance string) (domain.Severity, bool) {
	key := strings.ToLower(strings.TrimSpace(substance))
	s, ok := t[key]

	return s, ok
}

// Rank constants represent the ordinal position of each severity level.
// Higher values indicate greater danger.
const (
	rankUnknown  = -1
	rankInfo     = 0
	rankLow      = 1
	rankMedium   = 2
	rankHigh     = 3
	rankCritical = 4
)

// severityRank returns the numeric rank for a Severity value.
// An unrecognised value returns rankUnknown (-1).
func severityRank(s domain.Severity) int {
	switch s {
	case domain.SeverityInfo:
		return rankInfo
	case domain.SeverityLow:
		return rankLow
	case domain.SeverityMedium:
		return rankMedium
	case domain.SeverityHigh:
		return rankHigh
	case domain.SeverityCritical:
		return rankCritical
	default:
		return rankUnknown
	}
}
