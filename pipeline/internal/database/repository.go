package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jonesrussell/north-cloud/pipeline/internal/domain"
)

// Repository handles database operations for pipeline events.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new repository with the given database connection.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Ping checks database connectivity.
func (r *Repository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// UpsertArticle inserts or updates an article record.
func (r *Repository) UpsertArticle(ctx context.Context, article *domain.Article) error {
	query := `
		INSERT INTO articles (url, url_hash, domain, source_name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (url) DO NOTHING
	`

	_, err := r.db.ExecContext(ctx, query,
		article.URL,
		article.URLHash,
		article.Domain,
		article.SourceName,
	)
	if err != nil {
		return fmt.Errorf("upsert article: %w", err)
	}

	return nil
}

// InsertEvent inserts a pipeline event. Returns nil on idempotent duplicate.
func (r *Repository) InsertEvent(ctx context.Context, event *domain.PipelineEvent) error {
	metadataJSON, marshalErr := json.Marshal(event.Metadata)
	if marshalErr != nil {
		return fmt.Errorf("marshal metadata: %w", marshalErr)
	}

	query := `
		INSERT INTO pipeline_events
			(article_url, stage, occurred_at, service_name, metadata, metadata_schema_version, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (idempotency_key, occurred_at) DO NOTHING
		RETURNING id
	`

	var id int64
	scanErr := r.db.QueryRowContext(ctx, query,
		event.ArticleURL,
		string(event.Stage),
		event.OccurredAt,
		event.ServiceName,
		metadataJSON,
		event.MetadataSchemaVersion,
		event.IdempotencyKey,
	).Scan(&id)

	if scanErr == sql.ErrNoRows {
		// Idempotent duplicate — not an error
		return nil
	}
	if scanErr != nil {
		return fmt.Errorf("insert event: %w", scanErr)
	}

	event.ID = id
	return nil
}

// GetFunnel returns aggregated funnel stages for a time range.
func (r *Repository) GetFunnel(ctx context.Context, from, to time.Time) ([]domain.FunnelStage, error) {
	query := `
		SELECT
			pe.stage,
			COUNT(*) AS count,
			COUNT(DISTINCT pe.article_url) AS unique_articles
		FROM pipeline_events pe
		JOIN stage_ordering so ON pe.stage = so.stage
		WHERE pe.occurred_at >= $1 AND pe.occurred_at < $2
		GROUP BY pe.stage, so.sort_order
		ORDER BY so.sort_order
	`

	rows, queryErr := r.db.QueryContext(ctx, query, from, to)
	if queryErr != nil {
		return nil, fmt.Errorf("query funnel: %w", queryErr)
	}
	defer rows.Close()

	var stages []domain.FunnelStage
	for rows.Next() {
		var s domain.FunnelStage
		if scanErr := rows.Scan(&s.Name, &s.Count, &s.UniqueArticles); scanErr != nil {
			return nil, fmt.Errorf("scan funnel row: %w", scanErr)
		}
		stages = append(stages, s)
	}

	if closeErr := rows.Err(); closeErr != nil {
		return nil, fmt.Errorf("funnel rows: %w", closeErr)
	}

	return stages, nil
}
