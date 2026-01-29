package api

import (
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

const (
	// Priority constants for dashboard to database conversion
	priorityHigh            = 10
	priorityNormal          = 5
	priorityLow             = 1
	priorityHighThreshold   = 8
	priorityNormalThreshold = 4
)

// RuleResponse represents a classification rule response for the dashboard.
type RuleResponse struct {
	ID       int      `json:"id"`
	Topic    string   `json:"topic"` // Maps from topic_name
	Keywords []string `json:"keywords"`
	Pattern  *string  `json:"pattern,omitempty"` // Optional regex pattern
	Priority string   `json:"priority"`          // "high", "normal", "low"
	Enabled  bool     `json:"enabled"`
}

// RulesListResponse represents a list of rules with metadata.
type RulesListResponse struct {
	Rules []RuleResponse `json:"rules"`
	Total int            `json:"total"`
}

// CreateRuleRequest represents a request to create a rule.
type CreateRuleRequest struct {
	Topic    string   `binding:"required" json:"topic"`
	Keywords []string `binding:"required" json:"keywords"`
	Pattern  *string  `json:"pattern"`
	Priority string   `json:"priority"` // "high", "normal", "low"
	Enabled  bool     `json:"enabled"`
}

// UpdateRuleRequest represents a request to update a rule.
type UpdateRuleRequest struct {
	Topic    string   `json:"topic"`
	Keywords []string `json:"keywords"`
	Pattern  *string  `json:"pattern"`
	Priority string   `json:"priority"`
	Enabled  *bool    `json:"enabled"`
}

// TestRuleRequest represents a request to test a rule against content.
type TestRuleRequest struct {
	Title string `json:"title"`
	Body  string `binding:"required" json:"body"`
}

// TestRuleResponse represents the result of testing a rule against content.
type TestRuleResponse struct {
	Matched         bool     `json:"matched"`
	Score           float64  `json:"score"`
	Coverage        float64  `json:"coverage"`
	MatchCount      int      `json:"match_count"`
	UniqueMatches   int      `json:"unique_matches"`
	MatchedKeywords []string `json:"matched_keywords"`
}

// SourceReputationResponse represents a source reputation response for the dashboard.
type SourceReputationResponse struct {
	Name            string     `json:"name"`       // source_name
	Reputation      int        `json:"reputation"` // reputation_score
	Category        string     `json:"category"`
	TotalClassified int        `json:"total_classified"` // total_articles
	AvgQuality      float64    `json:"avg_quality"`      // average_quality_score
	LastUpdated     *time.Time `json:"last_updated"`     // last_classified_at
}

// SourcesListResponse represents a paginated list of sources.
type SourcesListResponse struct {
	Sources []SourceReputationResponse `json:"sources"`
	Total   int                        `json:"total"`
	Page    int                        `json:"page"`
	PerPage int                        `json:"per_page"`
}

// UpdateSourceRequest represents a request to update a source.
type UpdateSourceRequest struct {
	Category string `binding:"required,oneof=news blog government unknown" json:"category"`
}

// priorityStringToInt converts dashboard priority strings to database integer values.
// Dashboard uses: "high", "normal", "low"
// Database uses: 0-100 (higher = more important)
func priorityStringToInt(priority string) int {
	switch priority {
	case "high":
		return priorityHigh
	case "normal":
		return priorityNormal
	case "low":
		return priorityLow
	default:
		return priorityNormal // Default to normal
	}
}

// priorityIntToString converts database integer priorities to dashboard strings.
// Database uses: 0-100 (higher = more important)
// Dashboard uses: "high", "normal", "low"
func priorityIntToString(priority int) string {
	if priority >= priorityHighThreshold {
		return "high"
	}
	if priority >= priorityNormalThreshold {
		return "normal"
	}
	return "low"
}

// toRuleResponse converts a domain rule to an API response.
func toRuleResponse(rule *domain.ClassificationRule) RuleResponse {
	return RuleResponse{
		ID:       rule.ID,
		Topic:    rule.TopicName,
		Keywords: rule.Keywords,
		Pattern:  nil, // Not yet implemented in domain
		Priority: priorityIntToString(rule.Priority),
		Enabled:  rule.Enabled,
	}
}

// toSourceResponse converts a domain source reputation to an API response.
func toSourceResponse(source *domain.SourceReputation) SourceReputationResponse {
	return SourceReputationResponse{
		Name:            source.SourceName,
		Reputation:      source.ReputationScore,
		Category:        source.Category,
		TotalClassified: source.TotalArticles,
		AvgQuality:      source.AverageQualityScore,
		LastUpdated:     source.LastClassifiedAt,
	}
}

// ptr returns a pointer to a boolean value.
func ptr(b bool) *bool {
	return &b
}
