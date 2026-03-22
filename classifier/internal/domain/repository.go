package domain

import (
	"context"
	"time"
)

// SourceReputationListFilter holds pagination and filter params for listing source reputations.
type SourceReputationListFilter struct {
	Page      int
	PageSize  int
	SortBy    string // reputation, category, total_articles, last_classified_at
	SortOrder string // asc, desc
	Search    string // ILIKE on source_name
	Category  string // filter by category
}

// ClassificationStats represents overall classification statistics.
type ClassificationStats struct {
	TotalClassified     int            `json:"total_classified"`
	AvgQualityScore     float64        `json:"avg_quality_score"`
	CrimeRelated        int            `json:"crime_related"`
	AvgProcessingTimeMs float64        `json:"avg_processing_time_ms"`
	ContentTypes        map[string]int `json:"content_types"`
}

// TopicStat represents statistics for a single topic.
type TopicStat struct {
	Topic      string  `db:"topic"       json:"topic"`
	Count      int     `db:"count"       json:"count"`
	AvgQuality float64 `db:"avg_quality" json:"avg_quality,omitempty"`
}

// SourceStat represents statistics for a single source.
type SourceStat struct {
	SourceName string  `db:"source_name" json:"source_name"`
	Count      int     `db:"count"       json:"count"`
	AvgQuality float64 `db:"avg_quality" json:"avg_quality,omitempty"`
}

// RulesRepository defines operations for classification rules persistence.
type RulesRepository interface {
	Create(ctx context.Context, rule *ClassificationRule) error
	GetByID(ctx context.Context, id int) (*ClassificationRule, error)
	List(ctx context.Context, ruleType string, enabled *bool) ([]*ClassificationRule, error)
	Update(ctx context.Context, rule *ClassificationRule) error
	Delete(ctx context.Context, id int) error
}

// SourceReputationRepository defines operations for source reputation persistence.
type SourceReputationRepository interface {
	GetSource(ctx context.Context, sourceName string) (*SourceReputation, error)
	GetOrCreateSource(ctx context.Context, sourceName string) (*SourceReputation, error)
	UpdateSource(ctx context.Context, source *SourceReputation) error
	List(ctx context.Context, filter SourceReputationListFilter) ([]*SourceReputation, int, error)
}

// ClassificationHistoryRepository defines operations for classification history queries.
type ClassificationHistoryRepository interface {
	GetSourceStatsByName(ctx context.Context, sourceName string) (*SourceStat, error)
	GetStats(ctx context.Context, startDate *time.Time) (*ClassificationStats, error)
	GetTopicStats(ctx context.Context) ([]*TopicStat, error)
	GetSourceStats(ctx context.Context) ([]*SourceStat, error)
}
