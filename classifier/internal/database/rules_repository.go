package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/lib/pq"
)

// RulesRepository handles database operations for classification rules.
type RulesRepository struct {
	db *sqlx.DB
}

// NewRulesRepository creates a new rules repository.
func NewRulesRepository(db *sqlx.DB) *RulesRepository {
	return &RulesRepository{db: db}
}

// Create inserts a new rule into the database.
func (r *RulesRepository) Create(ctx context.Context, rule *domain.ClassificationRule) error {
	query := `
		INSERT INTO classification_rules (rule_name, rule_type, topic_name, keywords, min_confidence, enabled, priority)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		rule.RuleName,
		rule.RuleType,
		rule.TopicName,
		pq.Array(rule.Keywords),
		rule.MinConfidence,
		rule.Enabled,
		rule.Priority,
	).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create rule: %w", err)
	}

	return nil
}

// GetByID retrieves a rule by its ID.
func (r *RulesRepository) GetByID(ctx context.Context, id int) (*domain.ClassificationRule, error) {
	var rule domain.ClassificationRule
	query := `
		SELECT id, rule_name, rule_type, topic_name, keywords, min_confidence, enabled, priority,
		       created_at, updated_at
		FROM classification_rules
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&rule.ID,
		&rule.RuleName,
		&rule.RuleType,
		&rule.TopicName,
		pq.Array(&rule.Keywords),
		&rule.MinConfidence,
		&rule.Enabled,
		&rule.Priority,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("rule not found: %d", id)
		}
		return nil, fmt.Errorf("failed to get rule: %w", err)
	}

	return &rule, nil
}

// List retrieves all rules with optional filtering.
func (r *RulesRepository) List(ctx context.Context, ruleType string, enabled *bool) ([]*domain.ClassificationRule, error) {
	var rules []*domain.ClassificationRule
	var query string
	var args []any

	// Build query based on filters
	query = `
		SELECT id, rule_name, rule_type, topic_name, keywords, min_confidence, enabled, priority,
		       created_at, updated_at
		FROM classification_rules
	`

	var whereClauses []string
	argIndex := 1

	if ruleType != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("rule_type = $%d", argIndex))
		args = append(args, ruleType)
		argIndex++
	}

	if enabled != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("enabled = $%d", argIndex))
		args = append(args, *enabled)
		// argIndex intentionally not incremented - this is the last filter
		// If more filters are added in the future, they should increment argIndex
	}

	if len(whereClauses) > 0 {
		query += " WHERE "
		var builder strings.Builder
		for i, clause := range whereClauses {
			if i > 0 {
				builder.WriteString(" AND ")
			}
			builder.WriteString(clause)
		}
		query += builder.String()
	}

	query += " ORDER BY priority DESC, created_at ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list rules: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var rule domain.ClassificationRule
		if err = rows.Scan(
			&rule.ID,
			&rule.RuleName,
			&rule.RuleType,
			&rule.TopicName,
			pq.Array(&rule.Keywords),
			&rule.MinConfidence,
			&rule.Enabled,
			&rule.Priority,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan rule: %w", err)
		}
		rules = append(rules, &rule)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rules: %w", err)
	}

	return rules, nil
}

// Update updates an existing rule.
func (r *RulesRepository) Update(ctx context.Context, rule *domain.ClassificationRule) error {
	query := `
		UPDATE classification_rules
		SET rule_name = $1, rule_type = $2, topic_name = $3, keywords = $4,
		    min_confidence = $5, enabled = $6, priority = $7
		WHERE id = $8
		RETURNING updated_at
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		rule.RuleName,
		rule.RuleType,
		rule.TopicName,
		pq.Array(rule.Keywords),
		rule.MinConfidence,
		rule.Enabled,
		rule.Priority,
		rule.ID,
	).Scan(&rule.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("rule not found: %d", rule.ID)
		}
		return fmt.Errorf("failed to update rule: %w", err)
	}

	return nil
}

// Delete removes a rule from the database.
func (r *RulesRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM classification_rules WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete rule: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("rule not found: %d", id)
	}

	return nil
}

// Count returns the total number of rules.
func (r *RulesRepository) Count(ctx context.Context) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM classification_rules`

	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count rules: %w", err)
	}

	return count, nil
}
