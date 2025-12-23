package domain

import "time"

// ClassificationRule represents a rule for classifying content
type ClassificationRule struct {
	ID             int       `json:"id" db:"id"`
	RuleName       string    `json:"rule_name" db:"rule_name"`
	RuleType       string    `json:"rule_type" db:"rule_type"` // "content_type", "topic", "quality"
	TopicName      string    `json:"topic_name,omitempty" db:"topic_name"`
	Keywords       []string  `json:"keywords" db:"keywords"`
	MinConfidence  float64   `json:"min_confidence" db:"min_confidence"`
	Enabled        bool      `json:"enabled" db:"enabled"`
	Priority       int       `json:"priority" db:"priority"` // Higher priority rules evaluated first
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// RuleType constants
const (
	RuleTypeContentType = "content_type"
	RuleTypeTopic       = "topic"
	RuleTypeQuality     = "quality"
)

// SourceReputation represents source trustworthiness data
type SourceReputation struct {
	ID                  int        `json:"id" db:"id"`
	SourceName          string     `json:"source_name" db:"source_name"`
	SourceURL           string     `json:"source_url,omitempty" db:"source_url"`
	Category            string     `json:"category" db:"category"` // "news", "blog", "government", "unknown"
	ReputationScore     int        `json:"reputation_score" db:"reputation_score"` // 0-100
	TotalArticles       int        `json:"total_articles" db:"total_articles"`
	AverageQualityScore float64    `json:"average_quality_score" db:"average_quality_score"`
	SpamCount           int        `json:"spam_count" db:"spam_count"`
	LastClassifiedAt    *time.Time `json:"last_classified_at,omitempty" db:"last_classified_at"`
	CreatedAt           time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at" db:"updated_at"`
}

// ClassificationHistory represents audit trail for classifications
type ClassificationHistory struct {
	ID                   int        `json:"id" db:"id"`
	ContentID            string     `json:"content_id" db:"content_id"`
	ContentURL           string     `json:"content_url" db:"content_url"`
	SourceName           string     `json:"source_name" db:"source_name"`
	ContentType          string     `json:"content_type,omitempty" db:"content_type"`
	ContentSubtype       string     `json:"content_subtype,omitempty" db:"content_subtype"`
	QualityScore         int        `json:"quality_score,omitempty" db:"quality_score"`
	Topics               []string   `json:"topics,omitempty" db:"topics"`
	IsCrimeRelated       bool       `json:"is_crime_related" db:"is_crime_related"`
	SourceReputationScore int       `json:"source_reputation_score,omitempty" db:"source_reputation_score"`
	ClassifierVersion    string     `json:"classifier_version" db:"classifier_version"`
	ClassificationMethod string     `json:"classification_method" db:"classification_method"`
	ModelVersion         string     `json:"model_version,omitempty" db:"model_version"`
	Confidence           float64    `json:"confidence,omitempty" db:"confidence"`
	ProcessingTimeMs     int        `json:"processing_time_ms,omitempty" db:"processing_time_ms"`
	ClassifiedAt         time.Time  `json:"classified_at" db:"classified_at"`
}

// MLModel represents metadata about ML models
type MLModel struct {
	ID              int                    `json:"id" db:"id"`
	ModelName       string                 `json:"model_name" db:"model_name"`
	ModelVersion    string                 `json:"model_version" db:"model_version"`
	ModelType       string                 `json:"model_type" db:"model_type"` // "content_type", "topic", "quality"
	Accuracy        float64                `json:"accuracy,omitempty" db:"accuracy"`
	F1Score         float64                `json:"f1_score,omitempty" db:"f1_score"`
	PrecisionScore  float64                `json:"precision_score,omitempty" db:"precision_score"`
	RecallScore     float64                `json:"recall_score,omitempty" db:"recall_score"`
	TrainedAt       *time.Time             `json:"trained_at,omitempty" db:"trained_at"`
	FeatureSet      []string               `json:"feature_set,omitempty" db:"feature_set"`
	Hyperparameters map[string]interface{} `json:"hyperparameters,omitempty" db:"hyperparameters"`
	ModelPath       string                 `json:"model_path,omitempty" db:"model_path"`
	IsActive        bool                   `json:"is_active" db:"is_active"`
	Enabled         bool                   `json:"enabled" db:"enabled"`
	CreatedAt       time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at" db:"updated_at"`
}
