package domain

import (
	"net/url"
	"strings"
	"time"
)

// DomainState represents operator state for a discovered domain.
type DomainState struct {
	Domain           string     `db:"domain"             json:"domain"`
	Status           string     `db:"status"             json:"status"`
	Notes            *string    `db:"notes"              json:"notes,omitempty"`
	IgnoredAt        *time.Time `db:"ignored_at"         json:"ignored_at,omitempty"`
	IgnoredBy        *string    `db:"ignored_by"         json:"ignored_by,omitempty"`
	PromotedAt       *time.Time `db:"promoted_at"        json:"promoted_at,omitempty"`
	PromotedSourceID *string    `db:"promoted_source_id" json:"promoted_source_id,omitempty"`
	CreatedAt        time.Time  `db:"created_at"         json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"         json:"updated_at"`
}

// DomainAggregate holds aggregated stats for a discovered domain.
// Populated by the repository aggregation query, quality score computed in Go.
type DomainAggregate struct {
	Domain      string    `db:"domain"       json:"domain"`
	Status      string    `db:"status"       json:"status"`
	LinkCount   int       `db:"link_count"   json:"link_count"`
	SourceCount int       `db:"source_count" json:"source_count"`
	AvgDepth    float64   `db:"avg_depth"    json:"avg_depth"`
	FirstSeen   time.Time `db:"first_seen"   json:"first_seen"`
	LastSeen    time.Time `db:"last_seen"    json:"last_seen"`
	OKRatio     *float64  `db:"ok_ratio"     json:"ok_ratio"`
	HTMLRatio   *float64  `db:"html_ratio"   json:"html_ratio"`
	Notes       *string   `db:"notes"        json:"notes,omitempty"`

	// Computed in Go (not from DB)
	QualityScore     int      `db:"-" json:"quality_score"`
	ReferringSources []string `db:"-" json:"referring_sources"`
	IsExistingSource bool     `db:"-" json:"is_existing_source"`
}

// PathCluster represents a group of URLs sharing a common path prefix.
type PathCluster struct {
	Pattern string `json:"pattern"`
	Count   int    `json:"count"`
}

// Domain status constants.
const (
	DomainStatusActive    = "active"
	DomainStatusIgnored   = "ignored"
	DomainStatusReviewing = "reviewing"
	DomainStatusPromoted  = "promoted"
)

// Quality score weight constants.
const (
	qualityWeightOK      = 30
	qualityWeightHTML    = 30
	qualityWeightSources = 20
	qualityWeightRecency = 20
	sourceCountCap       = 5
	recencyDecayDays     = 30
	maxQualityScore      = 100
	hoursPerDay          = 24
)

// ComputeQualityScore calculates the quality score for a domain aggregate.
func (d *DomainAggregate) ComputeQualityScore() {
	score := 0.0

	if d.OKRatio != nil {
		score += *d.OKRatio * float64(qualityWeightOK)
	}

	if d.HTMLRatio != nil {
		score += *d.HTMLRatio * float64(qualityWeightHTML)
	}

	// Source count normalized (cap at sourceCountCap)
	sourceNorm := min(float64(d.SourceCount)/float64(sourceCountCap), 1.0)

	score += sourceNorm * float64(qualityWeightSources)

	// Recency normalized (decays over recencyDecayDays)
	daysSince := time.Since(d.LastSeen).Hours() / hoursPerDay
	recencyNorm := max(1.0-daysSince/float64(recencyDecayDays), 0)

	score += recencyNorm * float64(qualityWeightRecency)

	d.QualityScore = min(int(score), maxQualityScore)
}

// ExtractDomain extracts a normalized domain from a URL, stripping www. prefix.
func ExtractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	hostname := u.Hostname()

	return strings.TrimPrefix(hostname, "www.")
}
