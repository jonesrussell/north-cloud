package domain

import (
	"errors"
	"time"
)

// Severity represents the urgency level of a community alert.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Category is the alert discriminator. v1 supports harm_reduction only.
type Category string

const (
	CategoryHarmReduction Category = "harm_reduction"
)

// LifecycleState tracks whether an alert is still in effect.
type LifecycleState string

const (
	LifecycleActive    LifecycleState = "active"
	LifecycleRescinded LifecycleState = "rescinded"
)

// ParseQuality reflects the fidelity of the parse run.
type ParseQuality string

const (
	ParseClean    ParseQuality = "clean"
	ParseDegraded ParseQuality = "degraded"
	ParseFailed   ParseQuality = "failed"
)

// Alert is the canonical envelope for a community safety alert.
// It round-trips to JSON conforming to contracts/community-alert.schema.json.
type Alert struct {
	ID              string              `json:"id"`
	Category        Category            `json:"category"`
	Severity        Severity            `json:"severity"`
	Scope           []string            `json:"scope"`
	IssuedAt        time.Time           `json:"issued_at"`
	ExpiresAt       *time.Time          `json:"expires_at,omitempty"`
	LifecycleState  LifecycleState      `json:"lifecycle_state"`
	RescindedAt     *time.Time          `json:"rescinded_at,omitempty"`
	Title           string              `json:"title"`
	Summary         string              `json:"summary"`
	Hazard          Hazard              `json:"hazard"`
	Guidance        []string            `json:"guidance,omitempty"`
	Sources         []SourceAttribution `json:"sources"`
	RevisionHistory []Revision          `json:"revision_history,omitempty"`
	ParseQuality    ParseQuality        `json:"parse_quality"`
	CrawledAt       time.Time           `json:"crawled_at"`
	LastUpdatedAt   time.Time           `json:"last_updated_at"`
}

// SourceAttribution records where the alert was sourced from.
type SourceAttribution struct {
	SourceID        string   `json:"source_id"`
	SourceName      string   `json:"source_name"`
	URL             string   `json:"url"`
	AttributionText string   `json:"attribution_text,omitempty"`
	MediaLinks      []string `json:"media_links,omitempty"`
}

// Revision records a single change event in the alert's history.
type Revision struct {
	RevisionAt    time.Time `json:"revision_at"`
	RevisionKind  string    `json:"revision_kind"` // created|updated|rescinded|parse_degraded|parse_recovered
	ChangeSummary string    `json:"change_summary,omitempty"`
	ChangedFields []string  `json:"changed_fields,omitempty"`
}

// validSeverities is the set of accepted severity values.
var validSeverities = map[Severity]struct{}{
	SeverityInfo:     {},
	SeverityLow:      {},
	SeverityMedium:   {},
	SeverityHigh:     {},
	SeverityCritical: {},
}

// validCategories is the set of accepted category values.
var validCategories = map[Category]struct{}{
	CategoryHarmReduction: {},
}

// validLifecycleStates is the set of accepted lifecycle state values.
var validLifecycleStates = map[LifecycleState]struct{}{
	LifecycleActive:    {},
	LifecycleRescinded: {},
}

// validParseQualities is the set of accepted parse quality values.
var validParseQualities = map[ParseQuality]struct{}{
	ParseClean:    {},
	ParseDegraded: {},
	ParseFailed:   {},
}

// Validate checks that the alert satisfies the schema's required-field contract.
// Returns a combined error listing all violations.
func (a *Alert) Validate() error {
	var errs []error

	if a.ID == "" {
		errs = append(errs, errors.New("id is required"))
	}

	if _, ok := validCategories[a.Category]; !ok {
		errs = append(errs, errors.New("category is invalid or empty"))
	}

	if _, ok := validSeverities[a.Severity]; !ok {
		errs = append(errs, errors.New("severity is invalid or empty"))
	}

	if len(a.Scope) == 0 {
		errs = append(errs, errors.New("scope must contain at least one entry"))
	}

	if a.IssuedAt.IsZero() {
		errs = append(errs, errors.New("issued_at is required"))
	}

	if _, ok := validLifecycleStates[a.LifecycleState]; !ok {
		errs = append(errs, errors.New("lifecycle_state is invalid or empty"))
	}

	if a.Title == "" {
		errs = append(errs, errors.New("title is required"))
	}

	if a.Summary == "" {
		errs = append(errs, errors.New("summary is required"))
	}

	if len(a.Sources) == 0 {
		errs = append(errs, errors.New("sources must contain at least one entry"))
	}

	if _, ok := validParseQualities[a.ParseQuality]; !ok {
		errs = append(errs, errors.New("parse_quality is invalid or empty"))
	}

	return errors.Join(errs...)
}
