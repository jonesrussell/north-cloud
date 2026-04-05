package funding_test

import (
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
	fixture, err := os.ReadFile("testdata/otf_grants.html")
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(fixture)
	}))
	defer srv.Close()

	a := funding.New([]string{srv.URL})
	signals, err := a.Scan(context.Background())
	require.NoError(t, err)
	require.Len(t, signals, 2)

	first := signals[0]
	assert.Contains(t, first.Label, "TechStartup Inc")
	assert.Contains(t, first.Label, "Ontario Innovation Grant")
	assert.Equal(t, "TechStartup+Inc|Ontario+Innovation+Grant", first.ExternalID)
	assert.Equal(t, 70, first.SignalStrength)
	assert.Equal(t, "awarded", first.FundingStatus)
	assert.Equal(t, "Startup", first.OrganizationType)
	assert.Contains(t, first.SourceURL, "/funded-grants/123")
}

func TestFundingAdapter_PartialURLFailure(t *testing.T) {
	fixture, err := os.ReadFile("testdata/otf_grants.html")
	require.NoError(t, err)

	// First URL fails (404), second URL succeeds.
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(fixture)
	}))
	defer srv.Close()

	a := funding.New([]string{srv.URL + "/bad", srv.URL + "/good"})
	signals, err := a.Scan(context.Background())

	// Should return signals from the second URL despite the first failing.
	assert.Error(t, err, "should report the partial failure")
	assert.Len(t, signals, 2, "should still return signals from successful URLs")
}

func TestFundingAdapter_SkipsEmptyProgram(t *testing.T) {
	fixture, err := os.ReadFile("testdata/otf_grants_empty_program.html")
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(fixture)
	}))
	defer srv.Close()

	a := funding.New([]string{srv.URL})
	signals, err := a.Scan(context.Background())
	require.NoError(t, err)

	// Only the row with a non-empty program should be returned.
	require.Len(t, signals, 1)
	assert.Contains(t, signals[0].Label, "OrgWithProgram")
	assert.Contains(t, signals[0].Label, "Innovation Grant")
}

func TestFundingAdapter_EmptyPage(t *testing.T) {
	html := `<html><body><div class="view-content"></div></body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer srv.Close()

	a := funding.New([]string{srv.URL})
	signals, err := a.Scan(context.Background())
	require.NoError(t, err)
	assert.Empty(t, signals)
}
