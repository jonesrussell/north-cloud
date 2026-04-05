package jobs_test

import (
	"context"
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

const workBCFixture = `<html><body>
<div class="job-posting">
  <h2><a href="/jobs/12345">Cloud Infrastructure Technician</a></h2>
  <span class="employer">BC Public Service</span>
</div>
<div class="job-posting">
  <h2><a href="/jobs/12346">Administrative Assistant</a></h2>
  <span class="employer">City of Vancouver</span>
</div>
<div class="job-posting">
  <h2><a href="/jobs/12347">Platform Migration Analyst</a></h2>
  <span class="employer">BC Hydro</span>
</div>
</body></html>`

func TestWorkBC_Name(t *testing.T) {
	b := jobs.NewWorkBC("http://localhost", nil)
	assert.Equal(t, "workbc", b.Name())
}

func TestWorkBC_Fetch_StaticFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(workBCFixture))
	}))
	defer srv.Close()

	b := jobs.NewWorkBC(srv.URL, nil)
	postings, err := b.Fetch(context.Background())

	require.NoError(t, err)
	require.Len(t, postings, 3)

	assert.Equal(t, "Cloud Infrastructure Technician", postings[0].Title)
	assert.Equal(t, "BC Public Service", postings[0].Company)
	assert.Contains(t, postings[0].URL, "/jobs/12345")
	assert.Equal(t, "12345", postings[0].ID)
	assert.Equal(t, "government", postings[0].Sector)

	assert.Equal(t, "Administrative Assistant", postings[1].Title)
	assert.Equal(t, "City of Vancouver", postings[1].Company)

	assert.Equal(t, "Platform Migration Analyst", postings[2].Title)
	assert.Equal(t, "BC Hydro", postings[2].Company)
}

func TestWorkBC_Fetch_WithRenderer(t *testing.T) {
	renderer := &mockRenderer{html: workBCFixture}
	board := jobs.NewWorkBC("https://www.workbc.ca", renderer)
	postings, err := board.Fetch(context.Background())

	require.NoError(t, err)
	require.Len(t, postings, 3)
	assert.Equal(t, "Cloud Infrastructure Technician", postings[0].Title)
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
