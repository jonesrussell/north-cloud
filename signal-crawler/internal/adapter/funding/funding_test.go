package funding_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
	assert.Equal(t, "TechStartup Inc|Ontario Innovation Grant", first.ExternalID)
	assert.Equal(t, 70, first.SignalStrength)
	assert.Equal(t, "awarded", first.FundingStatus)
	assert.Equal(t, "Startup", first.OrganizationType)
	assert.Contains(t, first.SourceURL, "/funded-grants/123")
}

func TestFundingAdapter_EmptyPage(t *testing.T) {
	html := `<html><body><div class="view-content"></div></body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = strings.NewReader(html), w
		w.Write([]byte(html))
	}))
	defer srv.Close()

	a := funding.New([]string{srv.URL})
	signals, err := a.Scan(context.Background())
	require.NoError(t, err)
	assert.Empty(t, signals)
}
