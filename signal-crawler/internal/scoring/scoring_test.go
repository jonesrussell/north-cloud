package scoring_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/scoring"
	"github.com/stretchr/testify/assert"
)

func TestScore_ExistingKeywords(t *testing.T) {
	tests := []struct {
		text          string
		expectedScore int
		expectedMatch string
	}{
		{"We are looking for CTO to lead engineering", scoring.ScoreDirectAsk, "looking for cto"},
		{"Time to rebuild mvp from scratch", scoring.ScoreStrongSignal, "rebuild mvp"},
		{"Our legacy system needs work", scoring.ScoreWeakSignal, "legacy system"},
		{"Just a regular post about nothing", 0, ""},
	}

	for _, tt := range tests {
		score, matched := scoring.Score(tt.text)
		assert.Equal(t, tt.expectedScore, score, "text: %s", tt.text)
		assert.Equal(t, tt.expectedMatch, matched, "text: %s", tt.text)
	}
}

func TestScore_JobKeywords(t *testing.T) {
	tests := []struct {
		text          string
		expectedScore int
		expectedMatch string
	}{
		{"We're hiring platform engineer to rebuild our infra", scoring.ScoreDirectAsk, "hiring platform engineer"},
		{"Need cloud architect for AWS migration", scoring.ScoreDirectAsk, "need cloud architect"},
		{"Looking for devops lead to automate deployments", scoring.ScoreDirectAsk, "looking for devops"},
		{"Migrating monolith to microservices architecture", scoring.ScoreStrongSignal, "monolith to microservices"},
		{"Major cloud migration project starting Q2", scoring.ScoreStrongSignal, "cloud migration"},
		{"Infrastructure overhaul across all regions", scoring.ScoreStrongSignal, "infrastructure overhaul"},
		{"Platform modernization initiative underway", scoring.ScoreStrongSignal, "platform modernization"},
		{"Facing scaling challenges with current setup", scoring.ScoreWeakSignal, "scaling challenges"},
		{"We're growing engineering team rapidly", scoring.ScoreWeakSignal, "growing engineering team"},
		{"Time to start modernizing stack", scoring.ScoreWeakSignal, "modernizing stack"},
	}

	for _, tt := range tests {
		score, matched := scoring.Score(tt.text)
		assert.Equal(t, tt.expectedScore, score, "text: %s", tt.text)
		assert.Equal(t, tt.expectedMatch, matched, "text: %s", tt.text)
	}
}

func TestScore_HighestWins(t *testing.T) {
	text := "Hiring platform engineer for cloud migration project"
	score, _ := scoring.Score(text)
	assert.Equal(t, scoring.ScoreDirectAsk, score)
}

func TestMatchCount(t *testing.T) {
	text := "Hiring platform engineer for a cloud migration from a legacy system"

	assert.Equal(t, 3, scoring.MatchCount(text))
}

func TestMatchedPhrases(t *testing.T) {
	text := "Hiring platform engineer for a cloud migration from a legacy system"

	assert.ElementsMatch(t, []string{
		"hiring platform engineer",
		"cloud migration",
		"legacy system",
	}, scoring.MatchedPhrases(text))
}

func TestPassesAt(t *testing.T) {
	text := "Hiring platform engineer for a cloud migration project"

	ok, confidence, matches := scoring.PassesAt(text, 1)
	assert.True(t, ok)
	assert.InEpsilon(t, 0.80, confidence, 0.0001)
	assert.Equal(t, 2, matches)

	ok, confidence, matches = scoring.PassesAt("Hiring platform engineer", 2)
	assert.False(t, ok)
	assert.InDelta(t, 0.0, confidence, 0.0001)
	assert.Equal(t, 1, matches)
}

func TestPhrases_ReturnsCopy(t *testing.T) {
	phrases := scoring.Phrases()
	assert.Contains(t, phrases, "hiring platform engineer")

	phrases[0] = "mutated"
	fresh := scoring.Phrases()
	assert.NotEqual(t, "mutated", fresh[0])
}
