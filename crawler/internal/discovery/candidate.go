// Package discovery: candidate and enrichment types for the Source Candidate Pipeline.

package discovery

import "time"

// CandidateStatus is the status of a source candidate in the pipeline.
type CandidateStatus string

const (
	CandidateStatusPending    CandidateStatus = "pending"
	CandidateStatusApproved   CandidateStatus = "approved"
	CandidateStatusRejected   CandidateStatus = "rejected"
	CandidateStatusProcessing CandidateStatus = "processing" // created source, seeding frontier
)

// Enrichment holds metadata enrichment results for a source candidate.
// Rule-based fields (Category, TemplateHint) are deterministic given the same input;
// network-dependent fields (Title, RobotsTxtAllowed, FetchedAt) reflect state at enrichment time.
type Enrichment struct {
	Title              string    `json:"title"`
	FaviconURL         string    `json:"favicon_url"`
	RobotsTxtFetched   bool      `json:"robots_txt_fetched"`
	RobotsTxtAllowed   *bool     `json:"robots_txt_allowed,omitempty"` // nil if not fetched
	Category           string    `json:"category"`                     // inferred: news, blog, commerce, etc.
	TemplateHint       string    `json:"template_hint"`                // e.g. substack, wordpress
	ExtractionProfile  string    `json:"extraction_profile"`           // JSON or empty
	RateLimitSuggested string    `json:"rate_limit_suggested"`         // e.g. "10"
	RequiresConsent    bool      `json:"requires_consent"`
	AdultContent       bool      `json:"adult_content"`
	EnrichmentReason   string    `json:"enrichment_reason"` // explicit reason for audit
	FetchedAt          time.Time `json:"fetched_at"`
}

// SourceCandidate represents a candidate for a new source (before approval and creation).
// Used by the pipeline and stored in source_candidates table.
type SourceCandidate struct {
	ID                string          `json:"id"`
	CanonicalURL      string          `json:"canonical_url"`
	IdentityKey       string          `json:"identity_key"`
	ReferringSourceID string          `json:"referring_source_id"`
	Enrichment        *Enrichment     `json:"enrichment,omitempty"`
	RiskScore         float64         `json:"risk_score"`
	RiskReasons       []string        `json:"risk_reasons"`
	Status            CandidateStatus `json:"status"`
	ApprovedAt        *time.Time      `json:"approved_at,omitempty"`
	ApprovedBy        string          `json:"approved_by"` // user id or "rule:name"
	CreatedSourceID   string          `json:"created_source_id,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}
