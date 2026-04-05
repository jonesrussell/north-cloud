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
