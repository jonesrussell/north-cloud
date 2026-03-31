package scoring_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/scoring"
	"github.com/stretchr/testify/assert"
)

func TestScore_DirectAsk(t *testing.T) {
	cases := []string{
		"looking for CTO",
		"need developer",
		"hiring first engineer",
		"technical co-founder",
	}
	for _, text := range cases {
		score, matched := scoring.Score(text)
		assert.Equal(t, 90, score, "text: %q", text)
		assert.NotEmpty(t, matched, "text: %q", text)
	}
}

func TestScore_StrongSignal(t *testing.T) {
	cases := []string{
		"rebuild MVP",
		"rewriting our stack",
		"migrating to cloud",
		"scaling infrastructure",
	}
	for _, text := range cases {
		score, matched := scoring.Score(text)
		assert.Equal(t, 70, score, "text: %q", text)
		assert.NotEmpty(t, matched, "text: %q", text)
	}
}

func TestScore_WeakSignal(t *testing.T) {
	score, matched := scoring.Score("considering rewrite")
	assert.Equal(t, 40, score)
	assert.NotEmpty(t, matched)
}

func TestScore_NoMatch(t *testing.T) {
	score, matched := scoring.Score("Just launched our new product")
	assert.Equal(t, 0, score)
	assert.Empty(t, matched)
}

func TestScore_CaseInsensitive(t *testing.T) {
	score, matched := scoring.Score("LOOKING FOR CTO")
	assert.Equal(t, 90, score)
	assert.NotEmpty(t, matched)
}

func TestScore_HighestWins(t *testing.T) {
	score, matched := scoring.Score("Need developer to rebuild MVP")
	assert.Equal(t, 90, score)
	assert.NotEmpty(t, matched)
}
