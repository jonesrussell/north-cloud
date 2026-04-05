package jobs_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter/jobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHNHiring_Name(t *testing.T) {
	b := jobs.NewHNHiring("", "", 10)
	assert.Equal(t, "hn-hiring", b.Name())
}

func TestHNHiring_Fetch(t *testing.T) {
	mux := http.NewServeMux()

	// Algolia search endpoint — returns the latest "Who is hiring?" thread
	mux.HandleFunc("/api/v1/search", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"hits": []map[string]any{
				{"objectID": "9001", "title": "Ask HN: Who is hiring? (April 2026)"},
			},
		})
	})

	// Firebase thread item endpoint — returns kids (comment IDs)
	mux.HandleFunc("/v0/item/9001.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":   9001,
			"kids": []int{101, 102, 103},
		})
	})

	// Comment 101: matches scoring (platform engineer)
	mux.HandleFunc("/v0/item/101.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":   101,
			"text": "Acme Corp | Hiring platform engineer | Remote<p>We need someone to scale our infrastructure.",
		})
	})

	// Comment 102: no match
	mux.HandleFunc("/v0/item/102.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":   102,
			"text": "Boring Inc | Office Manager | NYC<p>Looking for someone to manage the office.",
		})
	})

	// Comment 103: matches scoring (cloud migration)
	mux.HandleFunc("/v0/item/103.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":   103,
			"text": "CloudCo | Cloud migration lead | SF<p>Leading our cloud migration to AWS.",
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	b := jobs.NewHNHiring(srv.URL, srv.URL, 10)
	postings, err := b.Fetch(context.Background())

	require.NoError(t, err)
	require.Len(t, postings, 3)

	assert.Equal(t, "Acme Corp", postings[0].Company)
	assert.Equal(t, "Hiring platform engineer", postings[0].Title)
	assert.Equal(t, "https://news.ycombinator.com/item?id=101", postings[0].URL)
	assert.Equal(t, "101", postings[0].ID)

	assert.Equal(t, "Boring Inc", postings[1].Company)
	assert.Equal(t, "Office Manager", postings[1].Title)

	assert.Equal(t, "CloudCo", postings[2].Company)
	assert.Equal(t, "Cloud migration lead", postings[2].Title)
}

func TestHNHiring_Fetch_NoHits(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/search", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"hits": []map[string]any{},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	b := jobs.NewHNHiring(srv.URL, srv.URL, 10)
	_, err := b.Fetch(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no hiring thread")
}
