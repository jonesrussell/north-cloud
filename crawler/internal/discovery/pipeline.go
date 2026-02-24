// Package discovery: Source Candidate Pipeline — process discovered URLs through resolution, enrichment, risk, approval, creation, and frontier seeding.

package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/frontier"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/apiclient"
)

const (
	decisionStageResolution = "resolution"
	decisionStageEnrichment = "enrichment"
	decisionStageRisk       = "risk_scoring"
	decisionStageApproval   = "approval"
	decisionStageCreation  = "source_creation"
	decisionStageFrontier  = "frontier_seed"
	originDiscovered       = "discovered"
	defaultFrontierDepth   = 0
	defaultFrontierPriority = 5
)

// DiscoveredURL is a URL discovered from a source (e.g. outlink) with the referring source id.
type DiscoveredURL struct {
	URL               string
	ReferringSourceID string
}

// CandidateStore persists and retrieves source candidates (implemented by database.CandidateRepository).
type CandidateStore interface {
	Create(ctx context.Context, c *SourceCandidate) error
	GetPendingByIdentityKey(ctx context.Context, identityKey string) (*SourceCandidate, error)
	ListPending(ctx context.Context, limit int) ([]*SourceCandidate, error)
	ListByStatus(ctx context.Context, status string, limit int) ([]*SourceCandidate, error)
	UpdateStatus(ctx context.Context, id, status string, approvedAt *time.Time, approvedBy, createdSourceID string) error
}

// DecisionLogStore appends decision log entries (implemented by database.DecisionLogRepository).
type DecisionLogStore interface {
	Insert(ctx context.Context, stage, reason string, inputs, outputs map[string]any) error
}

// FrontierSubmitter submits a URL to the frontier (implemented by callers using database.FrontierRepository).
type FrontierSubmitter interface {
	Submit(ctx context.Context, url, urlHash, host, sourceID, origin string, depth, priority int) error
}

// SourceCreator creates a source via source-manager API (implemented by callers using apiclient.Client).
type SourceCreator interface {
	CreateSource(ctx context.Context, source *apiclient.APISource) (*apiclient.APISource, error)
}

// DiscoveryConfigProvider provides allowlist, blocklist, caps, and global crawl budget for the pipeline.
type DiscoveryConfigProvider interface {
	Allowlist() []string
	Blocklist() []string
	MaxNewCandidatesPerRun() int
	GlobalCrawlBudgetPerDay() int
}

// Pipeline runs the Source Candidate Pipeline: resolution → enrichment → risk → dedup → (approval) → creation → frontier.
type Pipeline struct {
	resolver    *IdentityResolver
	enricher    *Enricher
	candidates  CandidateStore
	decisionLog DecisionLogStore
	frontier    FrontierSubmitter
	sourceAPI   SourceCreator
	config      DiscoveryConfigProvider
	log         Logger
}

// NewPipeline creates a new Source Candidate Pipeline.
func NewPipeline(
	resolver *IdentityResolver,
	enricher *Enricher,
	candidates CandidateStore,
	decisionLog DecisionLogStore,
	frontier FrontierSubmitter,
	sourceAPI SourceCreator,
	config DiscoveryConfigProvider,
	log Logger,
) *Pipeline {
	return &Pipeline{
		resolver:    resolver,
		enricher:    enricher,
		candidates:  candidates,
		decisionLog: decisionLog,
		frontier:    frontier,
		sourceAPI:   sourceAPI,
		config:      config,
		log:         log,
	}
}

// ProcessDiscoveredURLs runs the pipeline on a batch of discovered URLs. Only URLs from sources with
// allow_source_discovery and global discovery enabled should be passed. Respects allowlist/blocklist and MaxNewCandidatesPerRun.
func (p *Pipeline) ProcessDiscoveredURLs(ctx context.Context, urls []DiscoveredURL) error {
	allowlist := p.config.Allowlist()
	blocklist := p.config.Blocklist()
	maxCandidates := p.config.MaxNewCandidatesPerRun()
	candidatesCreated := 0

	for _, u := range urls {
		if u.URL == "" || u.ReferringSourceID == "" {
			continue
		}

		canonical, err := frontier.NormalizeURL(u.URL)
		if err != nil {
			_ = p.decisionLog.Insert(ctx, decisionStageResolution, "normalize_failed", map[string]any{"url": u.URL, "error": err.Error()}, nil)
			continue
		}

		if p.isBlocked(canonical, blocklist) {
			_ = p.decisionLog.Insert(ctx, decisionStageResolution, "blocklisted", map[string]any{"url": canonical}, nil)
			continue
		}
		if len(allowlist) > 0 && !p.isAllowlisted(canonical, allowlist) {
			_ = p.decisionLog.Insert(ctx, decisionStageResolution, "not_in_allowlist", map[string]any{"url": canonical}, nil)
			continue
		}

		resolved, err := p.resolver.Resolve(ctx, canonical, u.ReferringSourceID)
		if err != nil {
			_ = p.decisionLog.Insert(ctx, decisionStageResolution, "resolve_error", map[string]any{"url": canonical, "error": err.Error()}, nil)
			continue
		}

		if resolved.Kind == ResolvedExisting {
			_ = p.decisionLog.Insert(ctx, decisionStageResolution, resolved.Reason, map[string]any{"url": canonical, "identity_key": resolved.IdentityKey, "source_id": resolved.SourceID}, nil)
			if err := p.submitToFrontier(ctx, canonical, resolved.SourceID); err != nil {
				p.log.Info("Pipeline: frontier submit failed for existing source", "url", canonical, "source_id", resolved.SourceID, "error", err)
			}
			continue
		}

		// New or platform sub-source: ensure one candidate per identity_key
		existing, _ := p.candidates.GetPendingByIdentityKey(ctx, resolved.IdentityKey)
		if existing != nil {
			_ = p.decisionLog.Insert(ctx, decisionStageResolution, "duplicate_identity_key", map[string]any{"url": canonical, "identity_key": resolved.IdentityKey}, nil)
			continue
		}

		if maxCandidates > 0 && candidatesCreated >= maxCandidates {
			_ = p.decisionLog.Insert(ctx, decisionStageResolution, "max_candidates_per_run_reached", map[string]any{"url": canonical}, nil)
			continue
		}

		enrichment, enrichErr := p.enricher.Enrich(ctx, canonical)
		if enrichErr != nil {
			_ = p.decisionLog.Insert(ctx, decisionStageEnrichment, "enrich_error", map[string]any{"url": canonical, "error": enrichErr.Error()}, nil)
			continue
		}
		_ = p.decisionLog.Insert(ctx, decisionStageEnrichment, enrichment.EnrichmentReason, map[string]any{"url": canonical, "title": enrichment.Title, "category": enrichment.Category}, nil)

		riskScore, riskReasons := RiskScore(canonical, enrichment)
		_ = p.decisionLog.Insert(ctx, decisionStageRisk, fmt.Sprintf("risk_score=%.2f", riskScore), map[string]any{"url": canonical, "risk_score": riskScore, "reasons": riskReasons}, nil)

		// Robots pre-check: do not create candidate if disallowed
		if enrichment.RobotsTxtFetched && enrichment.RobotsTxtAllowed != nil && !*enrichment.RobotsTxtAllowed {
			_ = p.decisionLog.Insert(ctx, decisionStageRisk, "robots_txt_disallow", map[string]any{"url": canonical}, nil)
			continue
		}

		now := time.Now().UTC()
		c := &SourceCandidate{
			CanonicalURL:      canonical,
			IdentityKey:       resolved.IdentityKey,
			ReferringSourceID: u.ReferringSourceID,
			Enrichment:        enrichment,
			RiskScore:         riskScore,
			RiskReasons:       riskReasons,
			Status:            CandidateStatusPending,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		if err := p.candidates.Create(ctx, c); err != nil {
			_ = p.decisionLog.Insert(ctx, decisionStageRisk, "create_candidate_error", map[string]any{"url": canonical, "error": err.Error()}, nil)
			continue
		}
		candidatesCreated++
		_ = p.decisionLog.Insert(ctx, decisionStageApproval, "candidate_created_pending", map[string]any{"candidate_id": c.ID, "url": canonical, "identity_key": resolved.IdentityKey}, nil)
	}

	return nil
}

// ProcessApprovedCandidates creates sources and seeds the frontier for candidates with status "approved".
// Call after manual or rule-based approval. Logs creation and frontier seed to decision log.
func (p *Pipeline) ProcessApprovedCandidates(ctx context.Context, limit int) (processed int, err error) {
	list, listErr := p.candidates.ListByStatus(ctx, string(CandidateStatusApproved), limit)
	if listErr != nil {
		return 0, listErr
	}
	for _, c := range list {
		if c.Enrichment == nil {
			_ = p.decisionLog.Insert(ctx, decisionStageCreation, "missing_enrichment", map[string]any{"candidate_id": c.ID}, nil)
			continue
		}
		source := candidateToAPISource(c)
		created, createErr := p.sourceAPI.CreateSource(ctx, source)
		if createErr != nil {
			_ = p.decisionLog.Insert(ctx, decisionStageCreation, "create_source_error", map[string]any{"candidate_id": c.ID, "error": createErr.Error()}, nil)
			continue
		}
		_ = p.decisionLog.Insert(ctx, decisionStageCreation, "source_created", map[string]any{"candidate_id": c.ID, "source_id": created.ID}, nil)
		if submitErr := p.submitToFrontier(ctx, c.CanonicalURL, created.ID); submitErr != nil {
			p.log.Info("Pipeline: frontier seed failed after create", "candidate_id", c.ID, "source_id", created.ID, "error", submitErr)
		}
		_ = p.decisionLog.Insert(ctx, decisionStageFrontier, "frontier_seeded", map[string]any{"candidate_id": c.ID, "source_id": created.ID, "url": c.CanonicalURL}, nil)
		now := time.Now().UTC()
		_ = p.candidates.UpdateStatus(ctx, c.ID, string(CandidateStatusProcessing), &now, "pipeline", created.ID)
		processed++
	}
	return processed, nil
}

func (p *Pipeline) isBlocked(canonicalURL string, blocklist []string) bool {
	lower := strings.ToLower(canonicalURL)
	for _, b := range blocklist {
		if strings.TrimSpace(b) == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(strings.TrimSpace(b))) {
			return true
		}
	}
	return false
}

func (p *Pipeline) isAllowlisted(canonicalURL string, allowlist []string) bool {
	lower := strings.ToLower(canonicalURL)
	for _, a := range allowlist {
		if strings.TrimSpace(a) == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(strings.TrimSpace(a))) {
			return true
		}
	}
	return false
}

func (p *Pipeline) submitToFrontier(ctx context.Context, canonicalURL, sourceID string) error {
	urlHash, err := frontier.URLHash(canonicalURL)
	if err != nil {
		return err
	}
	host, err := frontier.ExtractHost(canonicalURL)
	if err != nil {
		return err
	}
	return p.frontier.Submit(ctx, canonicalURL, urlHash, host, sourceID, originDiscovered, defaultFrontierDepth, defaultFrontierPriority)
}

// candidateToAPISource builds an APISource for CreateSource from an approved candidate.
// PipelineX-ready: identity_key, template_hint, and extraction_profile are set when present.
func candidateToAPISource(c *SourceCandidate) *apiclient.APISource {
	name := c.CanonicalURL
	rateLimit := "10"
	if c.Enrichment != nil {
		if c.Enrichment.Title != "" {
			name = c.Enrichment.Title
		}
		if c.Enrichment.RateLimitSuggested != "" {
			rateLimit = c.Enrichment.RateLimitSuggested
		}
	}
	s := &apiclient.APISource{
		Name:      name,
		URL:       c.CanonicalURL,
		RateLimit: rateLimit,
		Enabled:   false,
	}
	if c.Enrichment != nil {
		if c.Enrichment.TemplateHint != "" {
			s.TemplateHint = &c.Enrichment.TemplateHint
		}
		if c.Enrichment.ExtractionProfile != "" {
			raw := json.RawMessage(c.Enrichment.ExtractionProfile)
			s.ExtractionProfile = &raw
		}
	}
	ik := c.IdentityKey
	s.IdentityKey = &ik
	return s
}
