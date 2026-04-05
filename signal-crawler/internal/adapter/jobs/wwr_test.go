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

const wwrFixture = `<html><body>
<section class="jobs"><ul>
  <li class="new-listing-container">
    <a class="listing-link--unlocked" href="/remote-jobs/platform-engineer-acme">
      <div class="new-listing">
        <div class="new-listing__header">
          <h3 class="new-listing__header__title">
            <span class="new-listing__header__title__text">Platform Engineer</span>
          </h3>
        </div>
        <p class="new-listing__company-name">Acme Corp<img src="logo.png"/></p>
      </div>
    </a>
  </li>
  <li class="new-listing-container">
    <a class="listing-link--unlocked" href="/remote-jobs/devops-lead-cloudco">
      <div class="new-listing">
        <div class="new-listing__header">
          <h3 class="new-listing__header__title">
            <span class="new-listing__header__title__text">DevOps Lead</span>
          </h3>
        </div>
        <p class="new-listing__company-name">CloudCo</p>
      </div>
    </a>
  </li>
  <li class="new-listing-container">
    <a class="listing-link--unlocked" href="/remote-jobs/office-manager-boring">
      <div class="new-listing">
        <div class="new-listing__header">
          <h3 class="new-listing__header__title">
            <span class="new-listing__header__title__text">Office Manager</span>
          </h3>
        </div>
        <p class="new-listing__company-name">Boring Co</p>
      </div>
    </a>
  </li>
</ul></section>
</body></html>`

func TestWWR_Name(t *testing.T) {
	b := jobs.NewWWR("http://localhost")
	assert.Equal(t, "wwr", b.Name())
}

func TestWWR_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(wwrFixture))
	}))
	defer srv.Close()

	b := jobs.NewWWR(srv.URL)
	postings, err := b.Fetch(context.Background())

	require.NoError(t, err)
	require.Len(t, postings, 3)

	assert.Equal(t, "Platform Engineer", postings[0].Title)
	assert.Equal(t, "Acme Corp", postings[0].Company)
	assert.Contains(t, postings[0].URL, "/remote-jobs/platform-engineer-acme")

	assert.Equal(t, "DevOps Lead", postings[1].Title)
	assert.Equal(t, "CloudCo", postings[1].Company)

	assert.Equal(t, "Office Manager", postings[2].Title)
}

func TestWWR_Fetch_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	b := jobs.NewWWR(srv.URL)
	_, err := b.Fetch(context.Background())
	assert.Error(t, err)
}
