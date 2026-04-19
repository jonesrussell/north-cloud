package funding_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter/funding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFundingAdapter_Name(t *testing.T) {
	a := funding.New(nil)
	assert.Equal(t, "funding", a.Name())
}

func TestFundingAdapter_Scan(t *testing.T) {
	fixture, err := os.ReadFile("testdata/otf_grants.csv")
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Write(fixture)
	}))
	defer srv.Close()

	a := funding.New([]string{srv.URL})
	signals, err := a.Scan(context.Background())
	require.NoError(t, err)

	// Only the 2 active, recently approved grants should be returned.
	// The "Closed" grant from 2020 should be filtered out.
	require.Len(t, signals, 2)

	first := signals[0]
	assert.Contains(t, first.Label, "TechStartup Inc")
	assert.Contains(t, first.Label, "Community Investments")
	assert.Equal(t, "496887GW144568", first.ExternalID)
	assert.Equal(t, 70, first.SignalStrength)
	assert.Equal(t, "awarded", first.FundingStatus)
	assert.Equal(t, "funding_win", first.SignalType)
	assert.Equal(t, "funding", first.SourceName)
	assert.Contains(t, first.Notes, "203400")
	assert.Contains(t, first.Notes, "Ottawa")
	assert.Equal(t, "TechStartup Inc", first.OrgName)
	assert.Equal(t, "techstartup", first.OrgNameNormalized, "explicit org wins over URL fallback")
}

func TestFundingAdapter_PartialURLFailure(t *testing.T) {
	fixture, err := os.ReadFile("testdata/otf_grants.csv")
	require.NoError(t, err)

	// First URL fails (404), second URL succeeds.
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Write(fixture)
	}))
	defer srv.Close()

	a := funding.New([]string{srv.URL + "/bad", srv.URL + "/good"})
	signals, err := a.Scan(context.Background())

	// Should return signals from the second URL despite the first failing.
	require.Error(t, err, "should report the partial failure")
	assert.Len(t, signals, 2, "should still return signals from successful URLs")
}

func TestFundingAdapter_EmptyCSV(t *testing.T) {
	fixture, err := os.ReadFile("testdata/otf_grants.csv")
	require.NoError(t, err)
	// Use only the header line (first line of fixture).
	header := string(fixture[:bytes.IndexByte(fixture, '\n')+1])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Write([]byte(header))
	}))
	defer srv.Close()

	a := funding.New([]string{srv.URL})
	signals, err := a.Scan(context.Background())
	require.NoError(t, err)
	assert.Empty(t, signals)
}

func TestFundingAdapter_FiltersClosedGrants(t *testing.T) {
	fixture, err := os.ReadFile("testdata/otf_grants.csv")
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Write(fixture)
	}))
	defer srv.Close()

	a := funding.New([]string{srv.URL})
	signals, err := a.Scan(context.Background())
	require.NoError(t, err)

	// The fixture has 3 rows: 2 Active (recent), 1 Closed (old).
	// Only active recent grants should appear.
	for _, sig := range signals {
		assert.NotContains(t, sig.Label, "Old Org", "closed grants should be filtered out")
	}
}
