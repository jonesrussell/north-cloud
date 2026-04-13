package feed_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/feed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResolver_Unknown(t *testing.T) {
	t.Helper()
	r := feed.NewResolver("bogus", "")
	assert.Nil(t, r)
}

func TestNewResolver_SEAOCKAN(t *testing.T) {
	t.Helper()
	r := feed.NewResolver("seao_ckan", "http://example.com")
	assert.NotNil(t, r)
}

func TestSEAOCKANResolver_Resolve(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"result": {
				"resources": [
					{"name": "hebdo_20260309_20260315", "url": "https://example.com/hebdo_20260309_20260315.json"},
					{"name": "hebdo_20260316_20260322", "url": "https://example.com/hebdo_20260316_20260322.json"},
					{"name": "archive_2025", "url": "https://example.com/archive_2025.csv"},
					{"name": "hebdo_20260302_20260308", "url": "https://example.com/hebdo_20260302_20260308.json"}
				]
			}
		}`))
	}))
	defer srv.Close()

	r := feed.NewResolver("seao_ckan", srv.URL)
	require.NotNil(t, r)

	urls, err := r.Resolve(context.Background())
	require.NoError(t, err)
	require.Len(t, urls, 1)
	assert.Equal(t, "https://example.com/hebdo_20260316_20260322.json", urls[0])
}

func TestSEAOCKANResolver_NoHebdoResources(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"result": {
				"resources": [
					{"name": "archive_2025", "url": "https://example.com/archive_2025.csv"}
				]
			}
		}`))
	}))
	defer srv.Close()

	r := feed.NewResolver("seao_ckan", srv.URL)
	require.NotNil(t, r)

	_, err := r.Resolve(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no hebdo JSON resources")
}

func TestSEAOCKANResolver_APIFailure(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := feed.NewResolver("seao_ckan", srv.URL)
	require.NotNil(t, r)

	_, err := r.Resolve(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}
