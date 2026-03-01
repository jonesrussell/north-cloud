package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
)

// Repository provides persistence operations for content and deliveries.
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// Ping verifies the database connection is alive.
func (r *Repository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// CreateContent stores a publish message as content in the database.
func (r *Repository) CreateContent(ctx context.Context, msg *domain.PublishMessage) error {
	images, err := json.Marshal(msg.Images)
	if err != nil {
		return fmt.Errorf("marshaling images: %w", err)
	}
	tags, err := json.Marshal(msg.Tags)
	if err != nil {
		return fmt.Errorf("marshaling tags: %w", err)
	}
	metadata, err := json.Marshal(msg.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	_, execErr := r.db.ExecContext(ctx, `
		INSERT INTO content (id, type, title, body, summary, url, images, tags, project, metadata, source, scheduled_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (id) DO NOTHING`,
		msg.ContentID, msg.Type, msg.Title, msg.Body, msg.Summary, msg.URL,
		images, tags, msg.Project, metadata, msg.Source, msg.ScheduledAt,
	)
	return execErr
}

// MarkContentPublished sets a content item's published flag to true.
func (r *Repository) MarkContentPublished(ctx context.Context, contentID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE content SET published = true WHERE id = $1`, contentID)
	return err
}

// GetDueScheduledContent returns scheduled content whose time has arrived.
func (r *Repository) GetDueScheduledContent(ctx context.Context, limit int) ([]domain.PublishMessage, error) {
	rows, err := r.db.QueryxContext(ctx, `
		SELECT id, type, title, body, summary, url, images, tags, project, metadata, source, scheduled_at
		FROM content
		WHERE scheduled_at <= NOW() AND published = false
		ORDER BY scheduled_at ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.PublishMessage
	for rows.Next() {
		msg, scanErr := scanPublishMessage(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		results = append(results, msg)
	}
	return results, rows.Err()
}

func scanPublishMessage(rows *sqlx.Rows) (domain.PublishMessage, error) {
	var msg domain.PublishMessage
	var images, tags, metadata []byte
	if err := rows.Scan(
		&msg.ContentID, &msg.Type, &msg.Title, &msg.Body, &msg.Summary, &msg.URL,
		&images, &tags, &msg.Project, &metadata, &msg.Source, &msg.ScheduledAt,
	); err != nil {
		return domain.PublishMessage{}, err
	}
	if err := json.Unmarshal(images, &msg.Images); err != nil {
		return domain.PublishMessage{}, fmt.Errorf("unmarshaling images: %w", err)
	}
	if err := json.Unmarshal(tags, &msg.Tags); err != nil {
		return domain.PublishMessage{}, fmt.Errorf("unmarshaling tags: %w", err)
	}
	if err := json.Unmarshal(metadata, &msg.Metadata); err != nil {
		return domain.PublishMessage{}, fmt.Errorf("unmarshaling metadata: %w", err)
	}
	return msg, nil
}

// CreateDelivery creates a pending delivery record for a content+platform+account combination.
func (r *Repository) CreateDelivery(
	ctx context.Context, contentID, platform, account string, maxAttempts int,
) (*domain.Delivery, error) {
	id := uuid.New().String()
	delivery := &domain.Delivery{
		ID:          id,
		ContentID:   contentID,
		Platform:    platform,
		Account:     account,
		Status:      domain.StatusPending,
		Attempts:    0,
		MaxAttempts: maxAttempts,
		CreatedAt:   time.Now(),
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO deliveries (id, content_id, platform, account, status, attempts, max_attempts, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (content_id, platform, account) DO NOTHING`,
		delivery.ID, delivery.ContentID, delivery.Platform, delivery.Account,
		delivery.Status, delivery.Attempts, delivery.MaxAttempts, delivery.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return delivery, nil
}

// UpdateDeliveryStatus sets a delivery's status and optional result/error fields.
func (r *Repository) UpdateDeliveryStatus(
	ctx context.Context, id string, status domain.DeliveryStatus,
	result *domain.DeliveryResult, errMsg *string,
) error {
	now := time.Now()
	var platformID, platformURL *string
	if result != nil {
		platformID = &result.PlatformID
		platformURL = &result.PlatformURL
	}

	query := `UPDATE deliveries SET status = $1, platform_id = $2, platform_url = $3, error = $4`
	args := []any{status, platformID, platformURL, errMsg}

	if status == domain.StatusDelivered {
		query += fmt.Sprintf(", delivered_at = $%d", len(args)+1)
		args = append(args, now)
	}
	if errMsg != nil {
		query += fmt.Sprintf(", last_error_at = $%d", len(args)+1)
		args = append(args, now)
	}

	query += fmt.Sprintf(" WHERE id = $%d", len(args)+1)
	args = append(args, id)

	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

// IncrementAttempts bumps the attempt count and schedules the next retry.
func (r *Repository) IncrementAttempts(ctx context.Context, id string, nextRetryAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE deliveries SET attempts = attempts + 1, status = 'retrying', next_retry_at = $1
		WHERE id = $2`, nextRetryAt, id)
	return err
}

// GetDueRetries returns deliveries that are due for retry.
func (r *Repository) GetDueRetries(ctx context.Context, limit int) ([]domain.Delivery, error) {
	var deliveries []domain.Delivery
	err := r.db.SelectContext(ctx, &deliveries, `
		SELECT * FROM deliveries
		WHERE status = 'retrying' AND next_retry_at <= NOW()
		ORDER BY next_retry_at ASC
		LIMIT $1`, limit)
	return deliveries, err
}

// GetDeliveriesByContentID returns all deliveries for a given content item.
func (r *Repository) GetDeliveriesByContentID(ctx context.Context, contentID string) ([]domain.Delivery, error) {
	var deliveries []domain.Delivery
	err := r.db.SelectContext(ctx, &deliveries, `
		SELECT * FROM deliveries WHERE content_id = $1 ORDER BY created_at`, contentID)
	return deliveries, err
}

// MarkDeliveryFailed permanently marks a delivery as failed.
func (r *Repository) MarkDeliveryFailed(ctx context.Context, id string, errMsg string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE deliveries SET status = 'failed', error = $1, last_error_at = $2
		WHERE id = $3`, errMsg, now, id)
	return err
}
