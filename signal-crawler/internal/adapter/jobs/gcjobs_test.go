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

// Fixture matches the real GC Jobs HTML structure: ol.posterInfo > li.searchResult
const gcJobsFixture = `<html><body>
<div id="searchResults" class="searchResults">
<ol start="1" class="posterInfo list-more-space">
  <li class="searchResult">
    <div>
      <strong>
        <a href="/psrs-srfp/applicant/page1800?poster=2405833">Cloud Infrastructure Analyst</a>
      </strong>
    </div>
    <div class="tableTable">
      Closing date: 2026-04-05
      <br>
      Treasury Board Secretariat
      <br>
      Ottawa (Ontario)
    </div>
    <hr class="searchJobHrLine">
  </li>
  <li class="searchResult">
    <div>
      <strong>
        <a href="/psrs-srfp/applicant/page1800?poster=2153439">Policy Advisor</a>
      </strong>
    </div>
    <div class="tableTable">
      Closing date: 2026-04-10
      <br>
      Finance Canada
      <br>
      Ottawa (Ontario)
    </div>
    <hr class="searchJobHrLine">
  </li>
  <li class="searchResult">
    <div>
      <strong>
        <a href="/psrs-srfp/applicant/page1800?poster=2200001">Platform Migration Specialist</a>
      </strong>
    </div>
    <div class="tableTable">
      Closing date: 2026-04-15
      <br>
      Shared Services Canada
      <br>
      Various Locations
    </div>
    <hr class="searchJobHrLine">
  </li>
</ol>
</div>
</body></html>`

func TestGCJobs_Name(t *testing.T) {
	b := jobs.NewGCJobs("http://localhost", nil)
	assert.Equal(t, "gcjobs", b.Name())
}

func TestGCJobs_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(gcJobsFixture))
	}))
	defer srv.Close()

	b := jobs.NewGCJobs(srv.URL, nil)
	postings, err := b.Fetch(context.Background())

	require.NoError(t, err)
	require.Len(t, postings, 3)

	assert.Equal(t, "Cloud Infrastructure Analyst", postings[0].Title)
	assert.Equal(t, "Treasury Board Secretariat", postings[0].Company)
	assert.Contains(t, postings[0].URL, "poster=2405833")
	assert.Equal(t, "government", postings[0].Sector)

	assert.Equal(t, "Policy Advisor", postings[1].Title)
	assert.Equal(t, "Finance Canada", postings[1].Company)
	assert.Equal(t, "government", postings[1].Sector)

	assert.Equal(t, "Platform Migration Specialist", postings[2].Title)
	assert.Equal(t, "Shared Services Canada", postings[2].Company)
}

func TestGCJobs_Fetch_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	b := jobs.NewGCJobs(srv.URL, nil)
	_, err := b.Fetch(context.Background())
	assert.Error(t, err)
}
