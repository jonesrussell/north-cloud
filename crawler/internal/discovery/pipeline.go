// Package discovery: Source Candidate Pipeline — process discovered URLs
// through resolution, enrichment, risk, approval, creation, and frontier seeding.

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
	decisionStageCreation   = "source_creation"
	decisionStageFrontier   = "frontier_seed"
	originDiscovered        = "discovered"
	defaultFrontierDepth    = 0
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

// Pipeline processes discovered URLs through two phases:
// (1) ProcessDiscoveredURLs: normalize → blocklist/allowlist → resolve identity → dedup → enrich → risk → robots check → create pending candidate.
// (2) ProcessApprovedCandidates: create source via API → seed frontier → mark processing.
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
// All dependencies are required; panics if any are nil (programming error at wiring time).
func NewPipeline(
	resolver *IdentityResolver,
	enricher *Enricher,
	candidates CandidateStore,
	decisionLog DecisionLogStore,
	frontierSub FrontierSubmitter,
	sourceAPI SourceCreator,
	config DiscoveryConfigProvider,
	log Logger,
) *Pipeline {
	if resolver == nil || enricher == nil || candidates == nil || decisionLog == nil ||
		frontierSub == nil || sourceAPI == nil || config == nil || log == nil {
		panic("discovery.NewPipeline: all dependencies are required (nil dependency detected)")
	}
	return &Pipeline{
		resolver:    resolver,
		enricher:    enricher,
		candidates:  candidates,
		decisionLog: decisionLog,
		frontier:    frontierSub,
		sourceAPI:   sourceAPI,
		config:      config,
		log:         log,
	}
}

// logDecision writes to the decision log and logs a warning if the write fails.
func (p *Pipeline) logDecision(ctx context.Context, stage, reason string, inputs map[string]any) {
	if err := p.decisionLog.Insert(ctx, stage, reason, inputs, nil); err != nil {
		p.log.Warn("decision log write failed",
			"stage", stage,
			"reason", reason,
			"error", err.Error(),
		)
	}
}

// ProcessDiscoveredURLs runs the pipeline on a batch of discovered URLs. Only URLs from sources with
// allow_source_discovery and global discovery enabled should be passed.
// Respects allowlist/blocklist and MaxNewCandidatesPerRun.
func (p *Pipeline) ProcessDiscoveredURLs(ctx context.Context, urls []DiscoveredURL) error {
	allowlist := p.config.Allowlist()
	blocklist := p.config.Blocklist()
	maxCandidates := p.config.MaxNewCandidatesPerRun()
	candidatesCreated := 0

	for _, u := range urls {
		if u.URL == "" || u.ReferringSourceID == "" {
			p.log.Info("Pipeline: skipping URL with empty required field",
				"url", u.URL,
				"referring_source_id", u.ReferringSourceID,
			)
			continue
		}

		canonical, err := frontier.NormalizeURL(u.URL)
		if err != nil {
			p.logDecision(ctx, decisionStageResolution, "normalize_failed",
				map[string]any{"url": u.URL, "error": err.Error()})
			continue
		}

		if !p.passesFilters(ctx, canonical, allowlist, blocklist) {
			continue
		}

		resolved, ok := p.resolveAndDedup(ctx, canonical, u.ReferringSourceID)
		if !ok {
			continue
		}

		if maxCandidates > 0 && candidatesCreated >= maxCandidates {
			p.logDecision(ctx, decisionStageResolution,
				"max_candidates_per_run_reached",
				map[string]any{"url": canonical})
			continue
		}

		if p.enrichAndCreateCandidate(ctx, canonical, resolved, u.ReferringSourceID) {
			candidatesCreated++
		}
	}

	p.log.Info("Pipeline: ProcessDiscoveredURLs completed",
		"total_urls", len(urls),
		"candidates_created", candidatesCreated,
	)

	return nil
}

// passesFilters returns true if the URL is not blocklisted and is allowlisted (when allowlist is set).
func (p *Pipeline) passesFilters(ctx context.Context, canonical string, allowlist, blocklist []string) bool {
	if p.isBlocked(canonical, blocklist) {
		p.logDecision(ctx, decisionStageResolution, "blocklisted",
			map[string]any{"url": canonical})
		return false
	}
	if len(allowlist) > 0 && !p.isAllowlisted(canonical, allowlist) {
		p.logDecision(ctx, decisionStageResolution, "not_in_allowlist",
			map[string]any{"url": canonical})
		return false
	}
	return true
}

// resolveAndDedup resolves identity and deduplicates. Returns the resolved identity and true if the URL
// is a new candidate, or nil and false if it was handled (existing source, duplicate, or error).
func (p *Pipeline) resolveAndDedup(
	ctx context.Context, canonical, referringSourceID string,
) (*ResolvedIdentity, bool) {
	resolved, err := p.resolver.Resolve(ctx, canonical, referringSourceID)
	if err != nil {
		p.logDecision(ctx, decisionStageResolution, "resolve_error",
			map[string]any{"url": canonical, "error": err.Error()})
		return nil, false
	}

	if resolved.Kind == ResolvedExisting {
		p.handleExistingSource(ctx, canonical, resolved)
		return nil, false
	}

	if !p.dedup(ctx, canonical, resolved.IdentityKey) {
		return nil, false
	}
	return resolved, true
}

// handleExistingSource logs the resolution and submits the URL to the frontier for the existing source.
func (p *Pipeline) handleExistingSource(ctx context.Context, canonical string, resolved *ResolvedIdentity) {
	existingInputs := map[string]any{
		"url":          canonical,
		"identity_key": resolved.IdentityKey,
		"source_id":    resolved.SourceID,
	}
	p.logDecision(ctx, decisionStageResolution, resolved.Reason, existingInputs)
	if submitErr := p.submitToFrontier(ctx, canonical, resolved.SourceID); submitErr != nil {
		p.log.Info("Pipeline: frontier submit failed for existing source",
			"url", canonical, "source_id", resolved.SourceID, "error", submitErr)
	}
}

// dedup checks that no pending candidate already exists for the identity key. Returns true if unique.
func (p *Pipeline) dedup(ctx context.Context, canonical, identityKey string) bool {
	existing, dedupeErr := p.candidates.GetPendingByIdentityKey(ctx, identityKey)
	if dedupeErr != nil {
		p.log.Error("Pipeline: dedup lookup failed, skipping URL to avoid duplicate",
			"url", canonical,
			"identity_key", identityKey,
			"error", dedupeErr.Error(),
		)
		p.logDecision(ctx, decisionStageResolution, "dedup_lookup_error",
			map[string]any{
				"url":          canonical,
				"identity_key": identityKey,
				"error":        dedupeErr.Error(),
			})
		return false
	}
	if existing != nil {
		p.logDecision(ctx, decisionStageResolution, "duplicate_identity_key",
			map[string]any{"url": canonical, "identity_key": identityKey})
		return false
	}
	return true
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
			p.logDecision(ctx, decisionStageCreation, "missing_enrichment", map[string]any{"candidate_id": c.ID})
			continue
		}
		source := candidateToAPISource(c)
		created, createErr := p.sourceAPI.CreateSource(ctx, source)
		if createErr != nil {
			p.logDecision(ctx, decisionStageCreation, "create_source_error", map[string]any{"candidate_id": c.ID, "error": createErr.Error()})
			continue
		}
		p.logDecision(ctx, decisionStageCreation, "source_created", map[string]any{"candidate_id": c.ID, "source_id": created.ID})
		if submitErr := p.submitToFrontier(ctx, c.CanonicalURL, created.ID); submitErr != nil {
			p.log.Warn("Pipeline: frontier seed failed after create",
				"candidate_id", c.ID, "source_id", created.ID, "error", submitErr)
			p.logDecision(ctx, decisionStageFrontier, "frontier_seed_failed",
				map[string]any{"candidate_id": c.ID, "source_id": created.ID, "error": submitErr.Error()})
		} else {
			p.logDecision(ctx, decisionStageFrontier, "frontier_seeded",
				map[string]any{"candidate_id": c.ID, "source_id": created.ID, "url": c.CanonicalURL})
		}
		now := time.Now().UTC()
		if updateErr := p.candidates.UpdateStatus(ctx, c.ID, string(CandidateStatusProcessing), &now, "pipeline", created.ID); updateErr != nil {
			p.log.Error("Pipeline: source created but status update failed, risk of duplicate on next run",
				"candidate_id", c.ID,
				"source_id", created.ID,
				"error", updateErr.Error(),
			)
		}
		processed++
	}
	return processed, nil
}

// enrichAndCreateCandidate runs enrichment, risk scoring, robots check, and candidate creation for a single URL.
// Returns true if a candidate was created.
func (p *Pipeline) enrichAndCreateCandidate(
	ctx context.Context,
	canonical string,
	resolved *ResolvedIdentity,
	referringSourceID string,
) bool {
	enrichment, enrichErr := p.enricher.Enrich(ctx, canonical)
	if enrichErr != nil {
		p.logDecision(ctx, decisionStageEnrichment, "enrich_error",
			map[string]any{"url": canonical, "error": enrichErr.Error()})
		return false
	}
	p.logDecision(ctx, decisionStageEnrichment, enrichment.EnrichmentReason,
		map[string]any{"url": canonical, "title": enrichment.Title, "category": enrichment.Category})

	riskScore, riskReasons := RiskScore(canonical, enrichment)
	p.logDecision(ctx, decisionStageRisk,
		fmt.Sprintf("risk_score=%.2f", riskScore),
		map[string]any{"url": canonical, "risk_score": riskScore, "reasons": riskReasons})

	// Robots pre-check: do not create candidate if disallowed
	if enrichment.RobotsTxtFetched && enrichment.RobotsTxtAllowed != nil && !*enrichment.RobotsTxtAllowed {
		p.logDecision(ctx, decisionStageRisk, "robots_txt_disallow",
			map[string]any{"url": canonical})
		return false
	}

	now := time.Now().UTC()
	c := &SourceCandidate{
		CanonicalURL:      canonical,
		IdentityKey:       resolved.IdentityKey,
		ReferringSourceID: referringSourceID,
		Enrichment:        enrichment,
		RiskScore:         riskScore,
		RiskReasons:       riskReasons,
		Status:            CandidateStatusPending,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if createErr := p.candidates.Create(ctx, c); createErr != nil {
		p.logDecision(ctx, decisionStageRisk, "create_candidate_error",
			map[string]any{"url": canonical, "error": createErr.Error()})
		return false
	}
	p.logDecision(ctx, decisionStageApproval, "candidate_created_pending",
		map[string]any{"candidate_id": c.ID, "url": canonical, "identity_key": resolved.IdentityKey})
	return true
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
	rateLimit := defaultRateLimitPerMinute
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
