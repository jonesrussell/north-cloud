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

const remoteOKFixture = `[
	{"legal": "https://remoteok.com/legal"},
	{
		"slug": "platform-engineer-acme",
		"company": "Acme Corp",
		"position": "Platform Engineer",
		"url": "https://remoteok.com/jobs/1",
		"id": "100",
		"description": "Hiring platform engineer."
	},
	{
		"slug": "office-manager-boring",
		"company": "Boring Co",
		"position": "Office Manager",
		"url": "https://remoteok.com/jobs/2",
		"id": "200",
		"description": "Manage office scheduling."
	},
	{
		"slug": "cloud-migration-cloudco",
		"company": "CloudCo",
		"position": "Cloud Migration Lead",
		"url": "https://remoteok.com/jobs/3",
		"id": "300",
		"description": "Lead cloud migration."
	}
]`

func TestRemoteOK_Name(t *testing.T) {
	b := jobs.NewRemoteOK("http://localhost")
	assert.Equal(t, "remoteok", b.Name())
}

func TestRemoteOK_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "north-cloud-signal-crawler/1.0", r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(remoteOKFixture))
	}))
	defer srv.Close()

	b := jobs.NewRemoteOK(srv.URL)
	postings, err := b.Fetch(context.Background())

	require.NoError(t, err)
	require.Len(t, postings, 3)

	assert.Equal(t, "Platform Engineer", postings[0].Title)
	assert.Equal(t, "Acme Corp", postings[0].Company)
	assert.Equal(t, "100", postings[0].ID)
	assert.Equal(t, "tech", postings[0].Sector)

	assert.Equal(t, "Office Manager", postings[1].Title)
	assert.Equal(t, "Cloud Migration Lead", postings[2].Title)
}

func TestRemoteOK_Fetch_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	b := jobs.NewRemoteOK(srv.URL)
	_, err := b.Fetch(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}
