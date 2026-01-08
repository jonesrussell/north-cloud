package domain

import "time"

// ClassificationRule represents a rule for classifying content
type ClassificationRule struct {
	ID            int       `db:"id"             json:"id"`
	RuleName      string    `db:"rule_name"      json:"rule_name"`
	RuleType      string    `db:"rule_type"      json:"rule_type"` // "content_type", "topic", "quality"
	TopicName     string    `db:"topic_name"     json:"topic_name,omitempty"`
	Keywords      []string  `db:"keywords"       json:"keywords"`
	MinConfidence float64   `db:"min_confidence" json:"min_confidence"`
	Enabled       bool      `db:"enabled"        json:"enabled"`
	Priority      int       `db:"priority"       json:"priority"` // Higher priority rules evaluated first
	CreatedAt     time.Time `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"     json:"updated_at"`
}

// RuleType constants
const (
	RuleTypeContentType = "content_type"
	RuleTypeTopic       = "topic"
	RuleTypeQuality     = "quality"
)

// SourceReputation represents source trustworthiness data
type SourceReputation struct {
	ID                  int        `db:"id"                    json:"id"`
	SourceName          string     `db:"source_name"           json:"source_name"`
	SourceURL           string     `db:"source_url"            json:"source_url,omitempty"`
	Category            string     `db:"category"              json:"category"`         // "news", "blog", "government", "unknown"
	ReputationScore     int        `db:"reputation_score"      json:"reputation_score"` // 0-100
	TotalArticles       int        `db:"total_articles"        json:"total_articles"`
	AverageQualityScore float64    `db:"average_quality_score" json:"average_quality_score"`
	SpamCount           int        `db:"spam_count"            json:"spam_count"`
	LastClassifiedAt    *time.Time `db:"last_classified_at"    json:"last_classified_at,omitempty"`
	CreatedAt           time.Time  `db:"created_at"            json:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at"            json:"updated_at"`
}

// ClassificationHistory represents audit trail for classifications
type ClassificationHistory struct {
	ID                    int       `db:"id"                      json:"id"`
	ContentID             string    `db:"content_id"              json:"content_id"`
	ContentURL            string    `db:"content_url"             json:"content_url"`
	SourceName            string    `db:"source_name"             json:"source_name"`
	ContentType           string    `db:"content_type"            json:"content_type,omitempty"`
	ContentSubtype        string    `db:"content_subtype"         json:"content_subtype,omitempty"`
	QualityScore          int       `db:"quality_score"           json:"quality_score,omitempty"`
	Topics                []string  `db:"topics"                  json:"topics,omitempty"`
	SourceReputationScore int       `db:"source_reputation_score" json:"source_reputation_score,omitempty"`
	ClassifierVersion     string    `db:"classifier_version"      json:"classifier_version"`
	ClassificationMethod  string    `db:"classification_method"   json:"classification_method"`
	ModelVersion          string    `db:"model_version"           json:"model_version,omitempty"`
	Confidence            float64   `db:"confidence"              json:"confidence,omitempty"`
	ProcessingTimeMs      int       `db:"processing_time_ms"      json:"processing_time_ms,omitempty"`
	ClassifiedAt          time.Time `db:"classified_at"           json:"classified_at"`
}

// MLModel represents metadata about ML models
type MLModel struct {
	ID              int                    `db:"id"              json:"id"`
	ModelName       string                 `db:"model_name"      json:"model_name"`
	ModelVersion    string                 `db:"model_version"   json:"model_version"`
	ModelType       string                 `db:"model_type"      json:"model_type"` // "content_type", "topic", "quality"
	Accuracy        float64                `db:"accuracy"        json:"accuracy,omitempty"`
	F1Score         float64                `db:"f1_score"        json:"f1_score,omitempty"`
	PrecisionScore  float64                `db:"precision_score" json:"precision_score,omitempty"`
	RecallScore     float64                `db:"recall_score"    json:"recall_score,omitempty"`
	TrainedAt       *time.Time             `db:"trained_at"      json:"trained_at,omitempty"`
	FeatureSet      []string               `db:"feature_set"     json:"feature_set,omitempty"`
	Hyperparameters map[string]any `db:"hyperparameters" json:"hyperparameters,omitempty"`
	ModelPath       string                 `db:"model_path"      json:"model_path,omitempty"`
	IsActive        bool                   `db:"is_active"       json:"is_active"`
	Enabled         bool                   `db:"enabled"         json:"enabled"`
	CreatedAt       time.Time              `db:"created_at"      json:"created_at"`
	UpdatedAt       time.Time              `db:"updated_at"      json:"updated_at"`
}
