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

const gcJobsFixture = `<html><body>
<article class="resultJobItem">
  <h3><a href="/en/job/1001">Cloud Infrastructure Analyst</a></h3>
  <div class="department">Treasury Board Secretariat</div>
</article>
<article class="resultJobItem">
  <h3><a href="/en/job/1002">Policy Advisor</a></h3>
  <div class="department">Finance Canada</div>
</article>
<article class="resultJobItem">
  <h3><a href="/en/job/1003">Platform Migration Specialist</a></h3>
  <div class="department">Shared Services Canada</div>
</article>
</body></html>`

func TestGCJobs_Name(t *testing.T) {
	b := jobs.NewGCJobs("http://localhost")
	assert.Equal(t, "gcjobs", b.Name())
}

func TestGCJobs_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(gcJobsFixture))
	}))
	defer srv.Close()

	b := jobs.NewGCJobs(srv.URL)
	postings, err := b.Fetch(context.Background())

	require.NoError(t, err)
	require.Len(t, postings, 3)

	assert.Equal(t, "Cloud Infrastructure Analyst", postings[0].Title)
	assert.Equal(t, "Treasury Board Secretariat", postings[0].Company)
	assert.Contains(t, postings[0].URL, "/en/job/1001")
	assert.Equal(t, "1001", postings[0].ID)
	assert.Equal(t, "government", postings[0].Sector)

	assert.Equal(t, "Policy Advisor", postings[1].Title)
	assert.Equal(t, "Finance Canada", postings[1].Company)
	assert.Equal(t, "government", postings[1].Sector)

	assert.Equal(t, "Platform Migration Specialist", postings[2].Title)
}

func TestGCJobs_Fetch_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	b := jobs.NewGCJobs(srv.URL)
	_, err := b.Fetch(context.Background())
	assert.Error(t, err)
}
