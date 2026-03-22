// Package discovery: type aliases for backward compatibility.
// Canonical types live in domain package (L0).

package discovery

import "github.com/jonesrussell/north-cloud/crawler/internal/domain"

// CandidateStatus is an alias for domain.CandidateStatus.
type CandidateStatus = domain.CandidateStatus

const (
	CandidateStatusPending    = domain.CandidateStatusPending
	CandidateStatusApproved   = domain.CandidateStatusApproved
	CandidateStatusRejected   = domain.CandidateStatusRejected
	CandidateStatusProcessing = domain.CandidateStatusProcessing
)

// Enrichment is an alias for domain.Enrichment.
type Enrichment = domain.Enrichment

// SourceCandidate is an alias for domain.SourceCandidate.
type SourceCandidate = domain.SourceCandidate
