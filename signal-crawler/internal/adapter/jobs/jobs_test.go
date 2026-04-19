package jobs_test

import (
	"context"
	"errors"
	"testing"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter/jobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubBoard struct {
	name     string
	postings []jobs.Posting
	err      error
}

func (s *stubBoard) Name() string { return s.name }
func (s *stubBoard) Fetch(_ context.Context) ([]jobs.Posting, error) {
	return s.postings, s.err
}

func TestAdapter_Name(t *testing.T) {
	a := jobs.New(nil, nil)
	assert.Equal(t, "jobs", a.Name())
}

func TestAdapter_Scan_ScoresAndFilters(t *testing.T) {
	log := infralogger.NewNop()

	boards := []jobs.Board{
		&stubBoard{
			name: "test-board",
			postings: []jobs.Posting{
				{Title: "Hiring platform engineer", Company: "Acme", URL: "https://example.com/1", ID: "1"},
				{Title: "Office Manager", Company: "Boring Co", URL: "https://example.com/2", ID: "2"},
				{Title: "Cloud migration lead needed", Company: "CloudCo", URL: "https://example.com/3", ID: "3"},
			},
		},
	}

	a := jobs.New(boards, log)
	signals, err := a.Scan(context.Background())

	require.NoError(t, err)
	assert.Len(t, signals, 2)

	assert.Equal(t, "Acme — Hiring platform engineer", signals[0].Label)
	assert.Equal(t, "test-board|1", signals[0].ExternalID)
	assert.Equal(t, 90, signals[0].SignalStrength)
	assert.Equal(t, "Acme", signals[0].OrgName)
	assert.Equal(t, "acme", signals[0].OrgNameNormalized)

	assert.Equal(t, "CloudCo — Cloud migration lead needed", signals[1].Label)
	assert.Equal(t, "test-board|3", signals[1].ExternalID)
	assert.Equal(t, 70, signals[1].SignalStrength)
	assert.Equal(t, "CloudCo", signals[1].OrgName)
	assert.Equal(t, "cloudco", signals[1].OrgNameNormalized)
}

func TestAdapter_Scan_URLFallback_WhenCompanyMissing(t *testing.T) {
	log := infralogger.NewNop()

	boards := []jobs.Board{
		&stubBoard{
			name: "anon-board",
			postings: []jobs.Posting{
				{Title: "Hiring platform engineer", URL: "https://acme-corp.com/jobs/42", ID: "42"},
			},
		},
	}

	a := jobs.New(boards, log)
	signals, err := a.Scan(context.Background())

	require.NoError(t, err)
	require.Len(t, signals, 1)
	assert.Empty(t, signals[0].OrgName, "raw OrgName stays empty when board omits company")
	assert.Equal(t, "acme", signals[0].OrgNameNormalized, "URL-apex fallback populates normalized (corp suffix stripped)")
}

func TestAdapter_Scan_BoardError_ContinuesOthers(t *testing.T) {
	log := infralogger.NewNop()

	boards := []jobs.Board{
		&stubBoard{name: "broken", err: errors.New("connection refused")},
		&stubBoard{
			name: "working",
			postings: []jobs.Posting{
				{Title: "Hiring platform engineer", Company: "Good Corp", URL: "https://example.com/4", ID: "4"},
			},
		},
	}

	a := jobs.New(boards, log)
	signals, err := a.Scan(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken")
	assert.Len(t, signals, 1)
	assert.Equal(t, "Good Corp — Hiring platform engineer", signals[0].Label)
}

func TestAdapter_Scan_DefaultSector(t *testing.T) {
	log := infralogger.NewNop()

	boards := []jobs.Board{
		&stubBoard{
			name: "test",
			postings: []jobs.Posting{
				{Title: "Hiring platform engineer", Company: "X", URL: "https://x.com/1", ID: "1"},
				{Title: "Hiring platform engineer", Company: "Y", URL: "https://y.com/2", ID: "2", Sector: "government"},
			},
		},
	}

	a := jobs.New(boards, log)
	signals, err := a.Scan(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "tech", signals[0].Sector)
	assert.Equal(t, "government", signals[1].Sector)
}
