package api

import (
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	defaultDomainsLimit  = 25
	defaultDomainsOffset = 0
	maxBulkDomains       = 100
)

// DiscoveredDomainsHandler handles discovered domain HTTP requests.
type DiscoveredDomainsHandler struct {
	aggregateRepo *database.DomainAggregateRepository
	stateRepo     *database.DomainStateRepository
	log           infralogger.Logger
}

// NewDiscoveredDomainsHandler creates a new discovered domains handler.
func NewDiscoveredDomainsHandler(
	aggregateRepo *database.DomainAggregateRepository,
	stateRepo *database.DomainStateRepository,
	log infralogger.Logger,
) *DiscoveredDomainsHandler {
	return &DiscoveredDomainsHandler{
		aggregateRepo: aggregateRepo,
		stateRepo:     stateRepo,
		log:           log,
	}
}

// ListDomains handles GET /api/v1/discovered-domains
func (h *DiscoveredDomainsHandler) ListDomains(c *gin.Context) {
	limit, offset := parseLimitOffset(c, defaultDomainsLimit, defaultDomainsOffset)

	filters := database.DomainListFilters{
		Status:    c.Query("status"),
		Search:    c.Query("search"),
		SortBy:    c.DefaultQuery("sort", "link_count"),
		SortOrder: c.DefaultQuery("order", "desc"),
		Limit:     limit,
		Offset:    offset,
	}

	domains, err := h.aggregateRepo.ListAggregates(c.Request.Context(), filters)
	if err != nil {
		h.log.Error("Failed to list domain aggregates", infralogger.Error(err))
		respondInternalError(c, "Failed to retrieve discovered domains")
		return
	}

	total, countErr := h.aggregateRepo.CountAggregates(c.Request.Context(), filters)
	if countErr != nil {
		h.log.Error("Failed to count domain aggregates", infralogger.Error(countErr))
		respondInternalError(c, "Failed to get total count")
		return
	}

	h.enrichDomainAggregates(c, domains)

	c.JSON(http.StatusOK, gin.H{
		"domains": domains,
		"total":   total,
	})
}

// enrichDomainAggregates computes quality scores and fetches referring sources for each domain.
func (h *DiscoveredDomainsHandler) enrichDomainAggregates(c *gin.Context, domains []*domain.DomainAggregate) {
	for _, d := range domains {
		d.ComputeQualityScore()

		sources, srcErr := h.aggregateRepo.GetReferringSources(c.Request.Context(), d.Domain)
		if srcErr == nil {
			d.ReferringSources = sources
		}
	}
}

// GetDomain handles GET /api/v1/discovered-domains/:domain
func (h *DiscoveredDomainsHandler) GetDomain(c *gin.Context) {
	domainName := c.Param("domain")

	filters := database.DomainListFilters{
		Search: domainName,
		Limit:  1,
	}

	domains, err := h.aggregateRepo.ListAggregates(c.Request.Context(), filters)
	if err != nil || len(domains) == 0 {
		respondNotFound(c, "Domain")
		return
	}

	d := domains[0]
	// Only return exact match
	if d.Domain != domainName {
		respondNotFound(c, "Domain")
		return
	}

	d.ComputeQualityScore()

	sources, srcErr := h.aggregateRepo.GetReferringSources(c.Request.Context(), d.Domain)
	if srcErr == nil {
		d.ReferringSources = sources
	}

	c.JSON(http.StatusOK, d)
}

// linkWithPath extends a discovered link with its extracted path.
type linkWithPath struct {
	*domain.DiscoveredLink
	Path string `json:"path"`
}

// ListDomainLinks handles GET /api/v1/discovered-domains/:domain/links
func (h *DiscoveredDomainsHandler) ListDomainLinks(c *gin.Context) {
	domainName := c.Param("domain")
	limit, offset := parseLimitOffset(c, defaultDomainsLimit, defaultDomainsOffset)

	links, total, err := h.aggregateRepo.ListLinksByDomain(
		c.Request.Context(), domainName, limit, offset,
	)
	if err != nil {
		h.log.Error("Failed to list domain links", infralogger.Error(err))
		respondInternalError(c, "Failed to retrieve domain links")
		return
	}

	clusters := computePathClusters(links)
	enriched := enrichLinksWithPaths(links)

	c.JSON(http.StatusOK, gin.H{
		"links":         enriched,
		"path_clusters": clusters,
		"total":         total,
	})
}

// enrichLinksWithPaths adds the extracted URL path to each link.
func enrichLinksWithPaths(links []*domain.DiscoveredLink) []linkWithPath {
	enriched := make([]linkWithPath, 0, len(links))

	for _, link := range links {
		enriched = append(enriched, linkWithPath{
			DiscoveredLink: link,
			Path:           extractPath(link.URL),
		})
	}

	return enriched
}

// UpdateDomainStateRequest represents the request body for updating domain state.
type UpdateDomainStateRequest struct {
	Status string  `binding:"required" json:"status"`
	Notes  *string `json:"notes"`
}

// UpdateDomainState handles PATCH /api/v1/discovered-domains/:domain/state
func (h *DiscoveredDomainsHandler) UpdateDomainState(c *gin.Context) {
	domainName := c.Param("domain")

	var req UpdateDomainStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if !isValidDomainStatus(req.Status) {
		respondBadRequest(c, "Invalid status: must be active, ignored, reviewing, or promoted")
		return
	}

	if err := h.stateRepo.Upsert(c.Request.Context(), domainName, req.Status, req.Notes); err != nil {
		h.log.Error("Failed to update domain state", infralogger.Error(err))
		respondInternalError(c, "Failed to update domain state")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Domain state updated",
		"domain":  domainName,
		"status":  req.Status,
	})
}

// BulkUpdateDomainStateRequest represents the request body for bulk domain state updates.
type BulkUpdateDomainStateRequest struct {
	Domains []string `binding:"required" json:"domains"`
	Status  string   `binding:"required" json:"status"`
	Notes   *string  `json:"notes"`
}

// BulkUpdateDomainState handles POST /api/v1/discovered-domains/bulk-state
func (h *DiscoveredDomainsHandler) BulkUpdateDomainState(c *gin.Context) {
	var req BulkUpdateDomainStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if len(req.Domains) == 0 {
		respondBadRequest(c, "At least one domain required")
		return
	}

	if len(req.Domains) > maxBulkDomains {
		respondBadRequest(c, "Maximum 100 domains per bulk operation")
		return
	}

	if !isValidDomainStatus(req.Status) {
		respondBadRequest(c, "Invalid status: must be active, ignored, reviewing, or promoted")
		return
	}

	count, err := h.stateRepo.BulkUpsert(c.Request.Context(), req.Domains, req.Status, req.Notes)
	if err != nil {
		h.log.Error("Failed to bulk update domain states", infralogger.Error(err))
		respondInternalError(c, "Failed to bulk update domain states")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Domain states updated",
		"updated": count,
	})
}

// isValidDomainStatus checks if a domain status is valid.
func isValidDomainStatus(status string) bool {
	switch status {
	case domain.DomainStatusActive, domain.DomainStatusIgnored,
		domain.DomainStatusReviewing, domain.DomainStatusPromoted:
		return true
	default:
		return false
	}
}

// computePathClusters groups URLs by their first path segment.
func computePathClusters(links []*domain.DiscoveredLink) []domain.PathCluster {
	counts := make(map[string]int)

	for _, link := range links {
		pattern := extractPathPattern(link.URL)
		counts[pattern]++
	}

	clusters := make([]domain.PathCluster, 0, len(counts))
	for pattern, count := range counts {
		clusters = append(clusters, domain.PathCluster{
			Pattern: pattern,
			Count:   count,
		})
	}

	slices.SortFunc(clusters, func(a, b domain.PathCluster) int {
		return b.Count - a.Count // descending by count
	})

	return clusters
}

// extractPathPattern extracts a path pattern from a URL.
// "/news/article/123" -> "/news/*"
// "/" -> "/"
func extractPathPattern(rawURL string) string {
	u, parseErr := url.Parse(rawURL)
	if parseErr != nil {
		return "/"
	}

	path := u.Path
	if path == "" || path == "/" {
		return "/"
	}

	// Get first path segment
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(segments) == 0 {
		return "/"
	}

	if len(segments) == 1 {
		return "/" + segments[0]
	}

	return "/" + segments[0] + "/*"
}

// extractPath extracts just the path from a URL.
func extractPath(rawURL string) string {
	u, parseErr := url.Parse(rawURL)
	if parseErr != nil {
		return "/"
	}

	if u.Path == "" {
		return "/"
	}

	return u.Path
}
