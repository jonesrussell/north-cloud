package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
)

// Repository provides persistence operations for content, deliveries, and accounts.
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

// GetContentByID loads a full PublishMessage from the database by content ID.
func (r *Repository) GetContentByID(ctx context.Context, contentID string) (*domain.PublishMessage, error) {
	rows, err := r.db.QueryxContext(ctx, `
		SELECT id, type, title, body, summary, url, images, tags, project, metadata, source, scheduled_at
		FROM content
		WHERE id = $1`, contentID)
	if err != nil {
		return nil, fmt.Errorf("querying content: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if rowErr := rows.Err(); rowErr != nil {
			return nil, fmt.Errorf("scanning content: %w", rowErr)
		}
		return nil, fmt.Errorf("content not found: %s", contentID)
	}

	msg, scanErr := scanPublishMessage(rows)
	if scanErr != nil {
		return nil, scanErr
	}
	return &msg, nil
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

	results := make([]domain.PublishMessage, 0, limit)
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
func (r *Repository) MarkDeliveryFailed(ctx context.Context, id, errMsg string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE deliveries SET status = 'failed', error = $1, last_error_at = $2
		WHERE id = $3`, errMsg, now, id)
	return err
}

// GetDeliveryByID returns a single delivery by its ID.
func (r *Repository) GetDeliveryByID(ctx context.Context, id string) (*domain.Delivery, error) {
	var delivery domain.Delivery
	err := r.db.GetContext(ctx, &delivery, `SELECT * FROM deliveries WHERE id = $1`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("getting delivery: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("getting delivery: %w", err)
	}
	return &delivery, nil
}

// ResetDeliveryForRetry resets a failed delivery to retrying with next_retry_at = NOW().
func (r *Repository) ResetDeliveryForRetry(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE deliveries SET status = 'retrying', next_retry_at = NOW(), error = NULL
		WHERE id = $1 AND status = 'failed'`, id)
	return err
}

// ListContent returns a paginated list of content items with delivery summaries.
func (r *Repository) ListContent(ctx context.Context, filter domain.ContentListFilter) ([]domain.ContentListItem, error) {
	query, args := buildListContentQuery(filter)
	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing content: %w", err)
	}
	defer rows.Close()

	items := make([]domain.ContentListItem, 0, filter.Limit)
	for rows.Next() {
		item, scanErr := scanContentListItem(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// CountContent returns the total count of content items matching the filter.
func (r *Repository) CountContent(ctx context.Context, filter domain.ContentListFilter) (int, error) {
	query, args := buildCountContentQuery(filter)
	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting content: %w", err)
	}
	return count, nil
}

func buildListContentQuery(filter domain.ContentListFilter) (query string, args []any) {
	query = `SELECT c.id, c.type, c.title, c.summary, c.url, c.project, c.source,
		c.published, c.scheduled_at, c.created_at,
		COALESCE(d.total, 0) AS total,
		COALESCE(d.pending, 0) AS pending,
		COALESCE(d.delivered, 0) AS delivered,
		COALESCE(d.failed, 0) AS failed,
		COALESCE(d.retrying, 0) AS retrying
	FROM content c
	LEFT JOIN (
		SELECT content_id,
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status = 'pending' OR status = 'publishing') AS pending,
			COUNT(*) FILTER (WHERE status = 'delivered') AS delivered,
			COUNT(*) FILTER (WHERE status = 'failed') AS failed,
			COUNT(*) FILTER (WHERE status = 'retrying') AS retrying
		FROM deliveries GROUP BY content_id
	) d ON d.content_id = c.id`

	const paginationParams = 2
	args = make([]any, 0, maxContentFilterConditions+paginationParams)
	conditions := buildContentConditions(filter, &args)
	if len(conditions) > 0 {
		query += " WHERE " + joinConditions(conditions)
	}
	query += " ORDER BY c.created_at DESC"
	limitPos := len(args) + 1
	offsetPos := limitPos + 1
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", limitPos, offsetPos)
	args = append(args, filter.Limit, filter.Offset)

	return query, args
}

func buildCountContentQuery(filter domain.ContentListFilter) (query string, args []any) {
	query = `SELECT COUNT(*) FROM content c
	LEFT JOIN (
		SELECT content_id,
			COUNT(*) FILTER (WHERE status = 'delivered') AS delivered,
			COUNT(*) FILTER (WHERE status = 'failed') AS failed
		FROM deliveries GROUP BY content_id
	) d ON d.content_id = c.id`

	args = make([]any, 0, maxContentFilterConditions)
	conditions := buildContentConditions(filter, &args)
	if len(conditions) > 0 {
		query += " WHERE " + joinConditions(conditions)
	}
	return query, args
}

const maxContentFilterConditions = 2

func buildContentConditions(filter domain.ContentListFilter, args *[]any) []string {
	conditions := make([]string, 0, maxContentFilterConditions)
	if filter.Type != "" {
		*args = append(*args, filter.Type)
		conditions = append(conditions, fmt.Sprintf("c.type = $%d", len(*args)))
	}
	if filter.Status != "" {
		switch filter.Status {
		case "delivered":
			conditions = append(conditions, "COALESCE(d.delivered, 0) > 0")
		case "failed":
			conditions = append(conditions, "COALESCE(d.failed, 0) > 0 AND COALESCE(d.delivered, 0) = 0")
		case "pending":
			conditions = append(conditions, "(COALESCE(d.total, 0) = 0 OR COALESCE(d.pending, 0) > 0)")
		}
	}
	return conditions
}

func joinConditions(conditions []string) string {
	return strings.Join(conditions, " AND ")
}

func scanContentListItem(rows *sqlx.Rows) (domain.ContentListItem, error) {
	var item domain.ContentListItem
	var summary domain.DeliverySummary
	if err := rows.Scan(
		&item.ID, &item.Type, &item.Title, &item.Summary, &item.URL,
		&item.Project, &item.Source, &item.Published, &item.ScheduledAt, &item.CreatedAt,
		&summary.Total, &summary.Pending, &summary.Delivered, &summary.Failed, &summary.Retrying,
	); err != nil {
		return domain.ContentListItem{}, fmt.Errorf("scanning content list item: %w", err)
	}
	item.DeliverySummary = &summary
	return item, nil
}

// ListAccounts returns all configured accounts. Credentials are never returned.
func (r *Repository) ListAccounts(ctx context.Context) ([]domain.Account, error) {
	rows, err := r.db.QueryxContext(ctx, `
		SELECT id, name, platform, project, enabled, credentials IS NOT NULL AS has_creds,
			token_expiry, created_at, updated_at
		FROM accounts
		ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing accounts: %w", err)
	}
	defer rows.Close()

	const initialAccountCapacity = 8
	accounts := make([]domain.Account, 0, initialAccountCapacity)
	for rows.Next() {
		var acct domain.Account
		var hasCreds bool
		if scanErr := rows.Scan(
			&acct.ID, &acct.Name, &acct.Platform, &acct.Project, &acct.Enabled,
			&hasCreds, &acct.TokenExpiry, &acct.CreatedAt, &acct.UpdatedAt,
		); scanErr != nil {
			return nil, fmt.Errorf("scanning account: %w", scanErr)
		}
		acct.CredentialsConfigured = hasCreds
		accounts = append(accounts, acct)
	}
	return accounts, rows.Err()
}

// GetAccountByID returns a single account by ID. Credentials are never returned.
func (r *Repository) GetAccountByID(ctx context.Context, id string) (*domain.Account, error) {
	var acct domain.Account
	var hasCreds bool
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, platform, project, enabled, credentials IS NOT NULL AS has_creds,
			token_expiry, created_at, updated_at
		FROM accounts WHERE id = $1`, id).Scan(
		&acct.ID, &acct.Name, &acct.Platform, &acct.Project, &acct.Enabled,
		&hasCreds, &acct.TokenExpiry, &acct.CreatedAt, &acct.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("getting account: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("getting account: %w", err)
	}
	acct.CredentialsConfigured = hasCreds
	return &acct, nil
}

// CreateAccount inserts a new account. Credentials should be pre-encrypted.
func (r *Repository) CreateAccount(
	ctx context.Context, id, name, platform, project string,
	enabled bool, encryptedCreds []byte, tokenExpiry *time.Time,
) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO accounts (id, name, platform, project, enabled, credentials, token_expiry)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, name, platform, project, enabled, encryptedCreds, tokenExpiry,
	)
	return err
}

// UpdateAccount updates account fields. Only non-nil fields are changed.
// Credentials (byte slice) are updated only if non-nil; passing nil leaves them unchanged.
func (r *Repository) UpdateAccount(
	ctx context.Context, id string,
	name, platform, project *string, enabled *bool,
	encryptedCreds []byte, tokenExpiry *time.Time,
) error {
	const maxUpdateAccountArgs = 7 // 6 optional fields + 1 ID
	query := "UPDATE accounts SET updated_at = NOW()"
	args := make([]any, 0, maxUpdateAccountArgs)

	if name != nil {
		args = append(args, *name)
		query += fmt.Sprintf(", name = $%d", len(args))
	}
	if platform != nil {
		args = append(args, *platform)
		query += fmt.Sprintf(", platform = $%d", len(args))
	}
	if project != nil {
		args = append(args, *project)
		query += fmt.Sprintf(", project = $%d", len(args))
	}
	if enabled != nil {
		args = append(args, *enabled)
		query += fmt.Sprintf(", enabled = $%d", len(args))
	}
	if encryptedCreds != nil {
		args = append(args, encryptedCreds)
		query += fmt.Sprintf(", credentials = $%d", len(args))
	}
	if tokenExpiry != nil {
		args = append(args, *tokenExpiry)
		query += fmt.Sprintf(", token_expiry = $%d", len(args))
	}

	args = append(args, id)
	query += fmt.Sprintf(" WHERE id = $%d", len(args))

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating account: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("updating account %s: %w", id, domain.ErrNotFound)
	}
	return nil
}

// DeleteAccount removes an account by ID.
func (r *Repository) DeleteAccount(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM accounts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting account: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("deleting account %s: %w", id, domain.ErrNotFound)
	}
	return nil
}

// GetAccountCredentials returns the raw encrypted credentials for an enabled account, looked up by name.
func (r *Repository) GetAccountCredentials(ctx context.Context, accountName string) ([]byte, error) {
	var creds []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT credentials FROM accounts WHERE name = $1 AND enabled = true`, accountName).Scan(&creds)
	if err != nil {
		return nil, fmt.Errorf("getting account credentials: %w", err)
	}
	return creds, nil
}
