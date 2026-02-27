package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

const (
	defaultDomainLimit     = 25
	defaultDomainSortField = "link_count"

	// httpStatusOKLower is the lower bound (inclusive) of HTTP 2xx success codes.
	httpStatusOKLower = 200

	// httpStatusOKUpper is the upper bound (inclusive) of HTTP 2xx success codes.
	httpStatusOKUpper = 299

	// argIdxIncrement is the positional argument increment for LIMIT/OFFSET binding.
	argIdxIncrement = 1

	// sortAsc and sortDesc are SQL sort order keywords.
	sortAsc  = "ASC"
	sortDesc = "DESC"
)

// DomainAggregateRepository handles domain-level aggregate queries.
type DomainAggregateRepository struct {
	db *sqlx.DB
}

// NewDomainAggregateRepository creates a new domain aggregate repository.
func NewDomainAggregateRepository(db *sqlx.DB) *DomainAggregateRepository {
	return &DomainAggregateRepository{db: db}
}

// DomainListFilters represents filtering options for listing domains.
type DomainListFilters struct {
	Status    string // Filter by domain state
	Search    string // ILIKE on domain name
	SortBy    string // link_count, last_seen, source_count, domain
	SortOrder string // asc, desc
	Limit     int
	Offset    int
}

// ListAggregates returns domain-level aggregates with optional filtering.
func (r *DomainAggregateRepository) ListAggregates(
	ctx context.Context,
	filters DomainListFilters,
) ([]*domain.DomainAggregate, error) {
	query, args := buildDomainAggregateQuery(filters)

	var results []*domain.DomainAggregate
	if err := r.db.SelectContext(ctx, &results, query, args...); err != nil {
		return nil, fmt.Errorf("list domain aggregates: %w", err)
	}

	if results == nil {
		results = []*domain.DomainAggregate{}
	}

	return results, nil
}

// CountAggregates returns the total number of distinct domains matching the filters.
func (r *DomainAggregateRepository) CountAggregates(
	ctx context.Context,
	filters DomainListFilters,
) (int, error) {
	query, args := buildDomainCountQuery(filters)

	var count int
	if err := r.db.GetContext(ctx, &count, query, args...); err != nil {
		return 0, fmt.Errorf("count domain aggregates: %w", err)
	}

	return count, nil
}

// GetReferringSources returns distinct source names for a given domain.
func (r *DomainAggregateRepository) GetReferringSources(
	ctx context.Context,
	domainName string,
) ([]string, error) {
	var sources []string

	query := `
		SELECT DISTINCT source_name
		FROM discovered_links
		WHERE domain = $1
		ORDER BY source_name
	`

	if err := r.db.SelectContext(ctx, &sources, query, domainName); err != nil {
		return nil, fmt.Errorf("get referring sources: %w", err)
	}

	if sources == nil {
		sources = []string{}
	}

	return sources, nil
}

// ListLinksByDomain returns paginated links for a specific domain.
func (r *DomainAggregateRepository) ListLinksByDomain(
	ctx context.Context,
	domainName string,
	limit, offset int,
) ([]*domain.DiscoveredLink, int, error) {
	if limit <= 0 {
		limit = defaultDomainLimit
	}

	links, linksErr := r.fetchDomainLinks(ctx, domainName, limit, offset)
	if linksErr != nil {
		return nil, 0, linksErr
	}

	total, countErr := r.countDomainLinks(ctx, domainName)
	if countErr != nil {
		return nil, 0, countErr
	}

	return links, total, nil
}

// fetchDomainLinks retrieves paginated links for a domain.
func (r *DomainAggregateRepository) fetchDomainLinks(
	ctx context.Context,
	domainName string,
	limit, offset int,
) ([]*domain.DiscoveredLink, error) {
	var links []*domain.DiscoveredLink

	query := `
		SELECT id, source_id, source_name, url, parent_url, depth, domain,
		       http_status, content_type, discovered_at, queued_at, status,
		       priority, created_at, updated_at
		FROM discovered_links
		WHERE domain = $1
		ORDER BY discovered_at DESC
		LIMIT $2 OFFSET $3
	`

	if err := r.db.SelectContext(ctx, &links, query, domainName, limit, offset); err != nil {
		return nil, fmt.Errorf("list links by domain: %w", err)
	}

	if links == nil {
		links = []*domain.DiscoveredLink{}
	}

	return links, nil
}

// countDomainLinks returns the total number of links for a domain.
func (r *DomainAggregateRepository) countDomainLinks(
	ctx context.Context,
	domainName string,
) (int, error) {
	var total int

	countQuery := `SELECT COUNT(*) FROM discovered_links WHERE domain = $1`
	if err := r.db.GetContext(ctx, &total, countQuery, domainName); err != nil {
		return 0, fmt.Errorf("count links by domain: %w", err)
	}

	return total, nil
}

// buildDomainAggregateQuery constructs the full SELECT query for listing aggregates.
func buildDomainAggregateQuery(filters DomainListFilters) (query string, args []any) {
	var whereClauses, havingClauses []string
	var argIdx int

	whereClauses, havingClauses, args, argIdx = buildDomainWhereClauses(filters)

	whereStr := joinClauses("WHERE", whereClauses)
	havingStr := joinClauses("HAVING", havingClauses)
	sortBy, sortOrder := normalizeDomainSort(filters.SortBy, filters.SortOrder)

	limit := filters.Limit
	if limit <= 0 {
		limit = defaultDomainLimit
	}

	offset := max(filters.Offset, 0)

	query = fmt.Sprintf(`
		SELECT
			dl.domain,
			COALESCE(ds.status, 'active') AS status,
			COUNT(*) AS link_count,
			COUNT(DISTINCT dl.source_id) AS source_count,
			AVG(dl.depth)::float8 AS avg_depth,
			MIN(dl.discovered_at) AS first_seen,
			MAX(dl.discovered_at) AS last_seen,
			%s,
			%s,
			ds.notes
		FROM discovered_links dl
		LEFT JOIN discovered_domain_states ds ON dl.domain = ds.domain
		%s
		GROUP BY dl.domain, ds.status, ds.notes
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, okRatioExpr(), htmlRatioExpr(),
		whereStr, havingStr, sortBy, sortOrder,
		argIdx, argIdx+argIdxIncrement)

	args = append(args, limit, offset)

	return query, args
}

// buildDomainCountQuery constructs a COUNT query for domain aggregates.
func buildDomainCountQuery(filters DomainListFilters) (query string, args []any) {
	var whereClauses, havingClauses []string

	whereClauses, havingClauses, args, _ = buildDomainWhereClauses(filters)

	whereStr := joinClauses("WHERE", whereClauses)
	havingStr := joinClauses("HAVING", havingClauses)

	query = fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT dl.domain
			FROM discovered_links dl
			LEFT JOIN discovered_domain_states ds ON dl.domain = ds.domain
			%s
			GROUP BY dl.domain, ds.status
			%s
		) sub
	`, whereStr, havingStr)

	return query, args
}

// buildDomainWhereClauses builds WHERE and HAVING clauses for domain queries.
func buildDomainWhereClauses(
	filters DomainListFilters,
) (where, having []string, args []any, nextArgIdx int) {
	argIdx := 1

	if filters.Search != "" {
		where = append(where, fmt.Sprintf("dl.domain ILIKE $%d", argIdx))
		args = append(args, "%"+filters.Search+"%")
		argIdx++
	}

	if filters.Status != "" {
		if filters.Status == domain.DomainStatusActive {
			// Active means no row in domain_states or status = 'active'
			having = append(having,
				fmt.Sprintf("COALESCE(ds.status, 'active') = $%d", argIdx))
		} else {
			having = append(having,
				fmt.Sprintf("ds.status = $%d", argIdx))
		}

		args = append(args, filters.Status)
		argIdx++
	}

	return where, having, args, argIdx
}

// normalizeDomainSort validates and normalizes sort parameters.
func normalizeDomainSort(sortBy, sortOrder string) (column, order string) {
	allowedSorts := map[string]string{
		"link_count":   "link_count",
		"source_count": "source_count",
		"last_seen":    "last_seen",
		"domain":       "dl.domain",
	}

	var ok bool

	column, ok = allowedSorts[sortBy]
	if !ok {
		column = allowedSorts[defaultDomainSortField]
	}

	order = strings.ToUpper(sortOrder)
	if order != sortAsc && order != sortDesc {
		order = sortDesc
	}

	return column, order
}

// joinClauses joins SQL clauses with a prefix keyword, or returns empty string if none.
func joinClauses(keyword string, clauses []string) string {
	if len(clauses) == 0 {
		return ""
	}

	return keyword + " " + strings.Join(clauses, " AND ")
}

// okRatioExpr returns the SQL expression for computing the HTTP 2xx success ratio.
func okRatioExpr() string {
	return fmt.Sprintf(`CASE WHEN COUNT(dl.http_status) > 0
			THEN COUNT(CASE WHEN dl.http_status BETWEEN %d AND %d THEN 1 END)::float8
				/ COUNT(dl.http_status)::float8
			ELSE NULL
		END AS ok_ratio`, httpStatusOKLower, httpStatusOKUpper)
}

// htmlRatioExpr returns the SQL expression for computing the HTML content type ratio.
func htmlRatioExpr() string {
	return `CASE WHEN COUNT(dl.content_type) > 0
			THEN COUNT(CASE WHEN dl.content_type LIKE 'text/html%%' THEN 1 END)::float8
				/ COUNT(dl.content_type)::float8
			ELSE NULL
		END AS html_ratio`
}
