package jobs_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter/jobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRenderer struct {
	html string
	err  error
}

func (m *mockRenderer) Render(_ context.Context, _ string) (string, error) {
	return m.html, m.err
}

const workBCAPIFixture = `{
  "result": [
    {
      "JobId": "abc123",
      "Title": "Cloud Infrastructure Technician",
      "EmployerName": "BC Public Service",
      "SalarySummary": "$80,000 - $95,000 annually",
      "ExternalSource": {
        "Source": [{"Url": "https://example.com/jobs/abc123", "Source": "example.com"}]
      }
    },
    {
      "JobId": "def456",
      "Title": "Administrative Assistant",
      "EmployerName": "City of Vancouver",
      "SalarySummary": "$45,000 - $55,000 annually",
      "ExternalSource": {
        "Source": [{"Url": "https://example.com/jobs/def456", "Source": "example.com"}]
      }
    },
    {
      "JobId": "ghi789",
      "Title": "Platform Migration Analyst",
      "EmployerName": "BC Hydro",
      "SalarySummary": "$90,000 - $110,000 annually",
      "ExternalSource": {
        "Source": [{"Url": "https://example.com/jobs/ghi789", "Source": "example.com"}]
      }
    }
  ],
  "count": 3
}`

func TestWorkBC_Name(t *testing.T) {
	b := jobs.NewWorkBC("http://localhost", nil)
	assert.Equal(t, "workbc", b.Name())
}

func TestWorkBC_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a POST with JSON body.
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Equal(t, float64(50), req["PageSize"])

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(workBCAPIFixture))
	}))
	defer srv.Close()

	b := jobs.NewWorkBC(srv.URL, nil)
	postings, err := b.Fetch(context.Background())

	require.NoError(t, err)
	require.Len(t, postings, 3)

	assert.Equal(t, "Cloud Infrastructure Technician", postings[0].Title)
	assert.Equal(t, "BC Public Service", postings[0].Company)
	assert.Equal(t, "https://example.com/jobs/abc123", postings[0].URL)
	assert.Equal(t, "abc123", postings[0].ID)
	assert.Equal(t, "government", postings[0].Sector)

	assert.Equal(t, "Administrative Assistant", postings[1].Title)
	assert.Equal(t, "City of Vancouver", postings[1].Company)

	assert.Equal(t, "Platform Migration Analyst", postings[2].Title)
	assert.Equal(t, "BC Hydro", postings[2].Company)
}

func TestWorkBC_Fetch_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	b := jobs.NewWorkBC(srv.URL, nil)
	_, err := b.Fetch(context.Background())
	assert.Error(t, err)
}
