// Package classifier provides content classification capabilities.
// rule_engine.go implements an Aho-Corasick based rule engine for O(n+m) keyword matching.
package classifier

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	ahocorasick "github.com/cloudflare/ahocorasick"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/telemetry"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// RuleMatch represents a matched rule with scoring details
type RuleMatch struct {
	Rule            *domain.ClassificationRule
	MatchCount      int      // Total keyword hits
	UniqueMatches   int      // Unique keywords matched
	Coverage        float64  // UniqueMatches / TotalKeywords
	Score           float64  // Final computed score
	MatchedKeywords []string // Which keywords matched (for debugging/testing)
}

// TrieRuleEngine uses Aho-Corasick for O(n+m) keyword matching.
// This is significantly faster than the naive O(r×k×n) approach when
// there are many rules with many keywords.
type TrieRuleEngine struct {
	mu        sync.RWMutex
	matcher   *ahocorasick.Matcher
	rules     []*domain.ClassificationRule
	keywords  []string                  // All keywords in order
	kwToRules map[string][]*ruleMapping // keyword -> rule mappings
	telemetry *telemetry.Provider
	logger    infralogger.Logger
}

type ruleMapping struct {
	rule         *domain.ClassificationRule
	keywordIndex int
}

// Rule engine constants
const (
	estimatedKeywordsPerRule = 20 // Used for initial slice capacity
)

// NewTrieRuleEngine builds the Aho-Corasick automaton from rules
func NewTrieRuleEngine(rules []*domain.ClassificationRule, logger infralogger.Logger, tp *telemetry.Provider) *TrieRuleEngine {
	enabledRules := make([]*domain.ClassificationRule, 0, len(rules))
	for _, rule := range rules {
		if rule.Enabled {
			enabledRules = append(enabledRules, rule)
		}
	}

	engine := &TrieRuleEngine{
		rules:     enabledRules,
		kwToRules: make(map[string][]*ruleMapping),
		telemetry: tp,
		logger:    logger,
	}
	// No lock needed in constructor - engine not yet shared
	engine.rebuildLocked()

	if logger != nil {
		logger.Info("trie rule engine initialized",
			infralogger.Int("rules", len(enabledRules)),
			infralogger.Int("keywords", len(engine.keywords)))
	}

	return engine
}

// rebuildLocked constructs the Aho-Corasick automaton.
// MUST be called with e.mu held (either read or write lock).
func (e *TrieRuleEngine) rebuildLocked() {
	e.keywords = make([]string, 0, len(e.rules)*estimatedKeywordsPerRule)
	e.kwToRules = make(map[string][]*ruleMapping)

	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}
		for idx, kw := range rule.Keywords {
			normalized := normalizeKeyword(kw)
			if normalized == "" {
				continue
			}
			e.keywords = append(e.keywords, normalized)
			e.kwToRules[normalized] = append(e.kwToRules[normalized], &ruleMapping{
				rule:         rule,
				keywordIndex: idx,
			})
		}
	}

	if len(e.keywords) > 0 {
		e.matcher = ahocorasick.NewStringMatcher(e.keywords)
	} else {
		e.matcher = nil
	}
}

// Match finds all matching rules in a single pass through the text.
// Returns rules sorted by priority (desc), then score (desc).
func (e *TrieRuleEngine) Match(title, body string) []RuleMatch {
	start := time.Now()

	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.matcher == nil {
		return nil
	}

	// Normalize input text
	text := normalizeText(title + " " + body)

	// Single pass through text - O(n + m)
	hits := e.matcher.Match([]byte(text))

	// Accumulate matches per rule
	ruleAccum := make(map[int]*matchAccumulator)

	for _, hitIndex := range hits {
		if hitIndex >= len(e.keywords) {
			continue
		}
		keyword := e.keywords[hitIndex]
		mappings := e.kwToRules[keyword]

		for _, m := range mappings {
			acc, exists := ruleAccum[m.rule.ID]
			if !exists {
				acc = &matchAccumulator{
					rule:            m.rule,
					matchedKeywords: make(map[int]bool),
					keywordTexts:    make([]string, 0),
				}
				ruleAccum[m.rule.ID] = acc
			}
			if !acc.matchedKeywords[m.keywordIndex] {
				acc.keywordTexts = append(acc.keywordTexts, keyword)
			}
			acc.matchedKeywords[m.keywordIndex] = true
			acc.totalHits++
		}
	}

	// Calculate scores and filter by confidence threshold
	results := make([]RuleMatch, 0, len(ruleAccum))
	for _, acc := range ruleAccum {
		totalKeywords := len(acc.rule.Keywords)
		if totalKeywords == 0 {
			continue
		}

		uniqueMatched := len(acc.matchedKeywords)
		coverage := float64(uniqueMatched) / float64(totalKeywords)

		// Log-scaled term frequency + coverage
		// This formula rewards both frequency AND breadth of matches
		logTF := math.Min(1.0, math.Log1p(float64(acc.totalHits))/tfNormalizationFactor)
		score := (logTF * tfWeight) + (coverage * coverageWeight)

		if score >= acc.rule.MinConfidence {
			results = append(results, RuleMatch{
				Rule:            acc.rule,
				MatchCount:      acc.totalHits,
				UniqueMatches:   uniqueMatched,
				Coverage:        coverage,
				Score:           score,
				MatchedKeywords: acc.keywordTexts,
			})
		}
	}

	// Record telemetry
	duration := time.Since(start)
	if e.telemetry != nil {
		e.telemetry.RecordRuleMatch(context.Background(), duration, len(e.rules), len(results))
	}

	// Sort by priority (desc), then score (desc)
	sort.Slice(results, func(i, j int) bool {
		if results[i].Rule.Priority != results[j].Rule.Priority {
			return results[i].Rule.Priority > results[j].Rule.Priority
		}
		return results[i].Score > results[j].Score
	})

	return results
}

// MatchWithDetails returns matches along with diagnostic information
func (e *TrieRuleEngine) MatchWithDetails(title, body string) ([]RuleMatch, MatchDetails) {
	start := time.Now()
	matches := e.Match(title, body)
	duration := time.Since(start)

	return matches, MatchDetails{
		RulesEvaluated: e.RuleCount(),
		RulesMatched:   len(matches),
		KeywordsTotal:  e.KeywordCount(),
		DurationMs:     duration.Milliseconds(),
	}
}

// MatchDetails holds diagnostic information about a match operation
type MatchDetails struct {
	RulesEvaluated int   `json:"rules_evaluated"`
	RulesMatched   int   `json:"rules_matched"`
	KeywordsTotal  int   `json:"keywords_total"`
	DurationMs     int64 `json:"duration_ms"`
}

// UpdateRules hot-reloads rules without restart.
// Thread-safe: acquires write lock to update rules and rebuild matcher atomically.
func (e *TrieRuleEngine) UpdateRules(rules []domain.ClassificationRule) {
	enabledRules := make([]*domain.ClassificationRule, 0, len(rules))
	for i := range rules {
		if rules[i].Enabled {
			enabledRules = append(enabledRules, &rules[i])
		}
	}

	// Acquire lock before updating rules to prevent race with Match()
	e.mu.Lock()
	e.rules = enabledRules
	e.rebuildLocked()
	keywordCount := len(e.keywords)
	e.mu.Unlock()

	if e.logger != nil {
		e.logger.Info("trie rule engine updated",
			infralogger.Int("rules", len(enabledRules)),
			infralogger.Int("keywords", keywordCount))
	}
}

// RuleCount returns the number of enabled rules
func (e *TrieRuleEngine) RuleCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.rules)
}

// KeywordCount returns total keywords across all enabled rules
func (e *TrieRuleEngine) KeywordCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.keywords)
}

// GetRules returns a copy of the current rules
func (e *TrieRuleEngine) GetRules() []*domain.ClassificationRule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*domain.ClassificationRule, len(e.rules))
	copy(result, e.rules)
	return result
}

// TestRule tests a single rule against content and returns match details
func (e *TrieRuleEngine) TestRule(ruleID int, title, body string) (*RuleMatch, bool) {
	matches := e.Match(title, body)
	for _, m := range matches {
		if m.Rule.ID == ruleID {
			return &m, true
		}
	}
	return nil, false
}

type matchAccumulator struct {
	rule            *domain.ClassificationRule
	matchedKeywords map[int]bool // keyword index -> matched
	keywordTexts    []string     // actual matched keyword strings
	totalHits       int
}

func normalizeKeyword(kw string) string {
	return strings.ToLower(strings.TrimSpace(kw))
}

func normalizeText(text string) string {
	// Lowercase and normalize unicode
	text = strings.ToLower(text)

	// Replace non-alphanumeric with spaces (preserves word boundaries)
	var result strings.Builder
	result.Grow(len(text))

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(r)
		} else {
			result.WriteByte(' ')
		}
	}

	return result.String()
}
