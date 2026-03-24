//nolint:testpackage // Testing internal storage requires same package access for helpers
package storage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestESClient creates an ES client backed by a local httptest server.
func newTestESClient(t *testing.T, handler http.HandlerFunc) *es.Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := es.NewClient(es.Config{
		Addresses: []string{srv.URL},
	})
	require.NoError(t, err)

	return client
}

// writeJSON encodes v as JSON to w with proper ES headers. Fails the test on error.
func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Elastic-Product", "Elasticsearch")

	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("failed to encode JSON response: %v", err)
	}
}

// writeErrorResponse writes an ES error response with the given status code and body.
func writeErrorResponse(w http.ResponseWriter, statusCode int, body string) {
	w.Header().Set("X-Elastic-Product", "Elasticsearch")
	w.WriteHeader(statusCode)

	_, _ = w.Write([]byte(body))
}

func TestNewElasticsearchStorage(t *testing.T) {
	t.Helper()

	client := newTestESClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	s := NewElasticsearchStorage(client)
	assert.NotNil(t, s)
	assert.NotNil(t, s.client)
}

func TestQueryRawContent_Success(t *testing.T) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	handler := func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"hits": map[string]any{
				"hits": []map[string]any{
					{
						"_index": "cbc_raw_content",
						"_id":    "doc-1",
						"_source": map[string]any{
							"id":                    "",
							"url":                   "https://cbc.ca/article",
							"source_name":           "cbc",
							"title":                 "Test Article",
							"raw_text":              "Body text",
							"classification_status": "pending",
							"crawled_at":            now.Format(time.RFC3339),
							"word_count":            100,
						},
					},
					{
						"_index": "globe_raw_content",
						"_id":    "doc-2",
						"_source": map[string]any{
							"id":                    "doc-2",
							"url":                   "https://globe.com/article",
							"source_name":           "globe",
							"title":                 "Another Article",
							"raw_text":              "More text",
							"classification_status": "pending",
							"crawled_at":            now.Format(time.RFC3339),
							"word_count":            200,
						},
					},
				},
			},
		}
		writeJSON(t, w, resp)
	}

	client := newTestESClient(t, handler)
	s := NewElasticsearchStorage(client)

	contents, err := s.QueryRawContent(context.Background(), "pending", 10)
	require.NoError(t, err)
	require.Len(t, contents, 2)

	// First hit: ID empty in _source, should be filled from _id
	assert.Equal(t, "doc-1", contents[0].ID)
	assert.Equal(t, "cbc_raw_content", contents[0].SourceIndex)
	assert.Equal(t, "cbc", contents[0].SourceName)
	assert.Equal(t, "Test Article", contents[0].Title)

	// Second hit: ID already in _source, should be preserved
	assert.Equal(t, "doc-2", contents[1].ID)
	assert.Equal(t, "globe_raw_content", contents[1].SourceIndex)
}

func TestQueryRawContent_EmptyResult(t *testing.T) {
	t.Helper()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"hits": map[string]any{
				"hits": []any{},
			},
		})
	}

	client := newTestESClient(t, handler)
	s := NewElasticsearchStorage(client)

	contents, err := s.QueryRawContent(context.Background(), "pending", 10)
	require.NoError(t, err)
	assert.Empty(t, contents)
}

func TestQueryRawContent_ESError(t *testing.T) {
	t.Helper()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		writeErrorResponse(w, http.StatusInternalServerError, `{"error":"internal server error"}`)
	}

	client := newTestESClient(t, handler)
	s := NewElasticsearchStorage(client)

	_, err := s.QueryRawContent(context.Background(), "pending", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error searching")
}

func TestIndexClassifiedContent_Success(t *testing.T) {
	t.Helper()

	var indexedBody []byte
	var requestPath string

	handler := func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		indexedBody, _ = io.ReadAll(r.Body)
		writeJSON(t, w, map[string]any{
			"result": "created",
			"_id":    "doc-1",
		})
	}

	client := newTestESClient(t, handler)
	s := NewElasticsearchStorage(client)

	content := &domain.ClassifiedContent{
		RawContent: domain.RawContent{
			ID:          "doc-1",
			URL:         "https://cbc.ca/article",
			SourceName:  "cbc",
			SourceIndex: "cbc_raw_content",
			RawText:     "Article body text",
		},
		ContentType:  domain.ContentTypeArticle,
		QualityScore: 75,
		Topics:       []string{"local_news"},
	}

	err := s.IndexClassifiedContent(context.Background(), content)
	require.NoError(t, err)

	// Verify correct index was used
	assert.Contains(t, requestPath, "cbc_classified_content")

	// Verify Body and Source aliases were set
	var indexed map[string]any
	require.NoError(t, json.Unmarshal(indexedBody, &indexed))
	assert.Equal(t, "Article body text", indexed["body"])
	assert.Equal(t, "https://cbc.ca/article", indexed["source"])

	// Verify classification status was updated
	assert.Equal(t, domain.StatusClassified, content.ClassificationStatus)
	assert.NotNil(t, content.ClassifiedAt)
}

func TestIndexClassifiedContent_ESError(t *testing.T) {
	t.Helper()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		writeErrorResponse(w, http.StatusInternalServerError, `{"error":"internal server error"}`)
	}

	client := newTestESClient(t, handler)
	s := NewElasticsearchStorage(client)

	content := &domain.ClassifiedContent{
		RawContent: domain.RawContent{
			ID:          "doc-1",
			SourceName:  "cbc",
			SourceIndex: "cbc_raw_content",
		},
	}

	err := s.IndexClassifiedContent(context.Background(), content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error indexing document")
}

func TestIndexClassifiedContent_InvalidIndex(t *testing.T) {
	t.Helper()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.WriteHeader(http.StatusOK)
	}

	client := newTestESClient(t, handler)
	s := NewElasticsearchStorage(client)

	content := &domain.ClassifiedContent{
		RawContent: domain.RawContent{
			ID:         "doc-1",
			SourceName: "", // Both empty = error
		},
	}

	err := s.IndexClassifiedContent(context.Background(), content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to determine classified index")
}

func TestBulkIndexClassifiedContent_EmptySlice(t *testing.T) {
	t.Helper()

	client := newTestESClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	s := NewElasticsearchStorage(client)

	err := s.BulkIndexClassifiedContent(context.Background(), nil)
	require.NoError(t, err)

	err = s.BulkIndexClassifiedContent(context.Background(), []*domain.ClassifiedContent{})
	require.NoError(t, err)
}

func TestBulkIndexClassifiedContent_Success(t *testing.T) {
	t.Helper()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"errors": false,
			"items": []map[string]any{
				{"index": map[string]any{"_index": "cbc_classified_content", "_id": "doc-1", "status": 201}},
				{"index": map[string]any{"_index": "globe_classified_content", "_id": "doc-2", "status": 201}},
			},
		})
	}

	client := newTestESClient(t, handler)
	s := NewElasticsearchStorage(client)

	contents := []*domain.ClassifiedContent{
		{
			RawContent: domain.RawContent{
				ID: "doc-1", SourceName: "cbc", SourceIndex: "cbc_raw_content",
				URL: "https://cbc.ca/1", RawText: "body1",
			},
		},
		{
			RawContent: domain.RawContent{
				ID: "doc-2", SourceName: "globe", SourceIndex: "globe_raw_content",
				URL: "https://globe.com/1", RawText: "body2",
			},
		},
	}

	err := s.BulkIndexClassifiedContent(context.Background(), contents)
	require.NoError(t, err)

	// Verify aliases were set
	assert.Equal(t, "body1", contents[0].Body)
	assert.Equal(t, "https://cbc.ca/1", contents[0].Source)
}

func TestBulkIndexClassifiedContent_ItemError(t *testing.T) {
	t.Helper()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"errors": true,
			"items": []map[string]any{
				{"index": map[string]any{
					"_index": "bad", "_id": "doc-1", "status": 400,
					"error": map[string]any{"type": "mapper_parsing_exception", "reason": "field error"},
				}},
			},
		})
	}

	client := newTestESClient(t, handler)
	s := NewElasticsearchStorage(client)

	contents := []*domain.ClassifiedContent{
		{RawContent: domain.RawContent{ID: "doc-1", SourceName: "cbc", SourceIndex: "cbc_raw_content"}},
	}

	err := s.BulkIndexClassifiedContent(context.Background(), contents)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 of 1 bulk items failed")
}

func TestBulkIndexClassifiedContent_InvalidIndex(t *testing.T) {
	t.Helper()

	client := newTestESClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	s := NewElasticsearchStorage(client)

	contents := []*domain.ClassifiedContent{
		{RawContent: domain.RawContent{ID: "doc-1", SourceName: ""}}, // Empty = error
	}

	err := s.BulkIndexClassifiedContent(context.Background(), contents)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to determine classified index")
}

func TestListRawContentIndices_Success(t *testing.T) {
	t.Helper()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"cbc_raw_content":   map[string]any{},
			"globe_raw_content": map[string]any{},
		})
	}

	client := newTestESClient(t, handler)
	s := NewElasticsearchStorage(client)

	indices, err := s.ListRawContentIndices(context.Background())
	require.NoError(t, err)
	assert.Len(t, indices, 2)
	assert.Contains(t, indices, "cbc_raw_content")
	assert.Contains(t, indices, "globe_raw_content")
}

func TestListRawContentIndices_ESError(t *testing.T) {
	t.Helper()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		writeErrorResponse(w, http.StatusInternalServerError, `{"error":"internal"}`)
	}

	client := newTestESClient(t, handler)
	s := NewElasticsearchStorage(client)

	_, err := s.ListRawContentIndices(context.Background())
	require.Error(t, err)
}

func TestTestConnection_Success(t *testing.T) {
	t.Helper()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"name":         "test-node",
			"cluster_name": "test-cluster",
			"version":      map[string]any{"number": "8.0.0"},
		})
	}

	client := newTestESClient(t, handler)
	s := NewElasticsearchStorage(client)

	err := s.TestConnection(context.Background())
	require.NoError(t, err)
}

func TestTestConnection_Error(t *testing.T) {
	t.Helper()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		writeErrorResponse(w, http.StatusServiceUnavailable, `{"error":"unavailable"}`)
	}

	client := newTestESClient(t, handler)
	s := NewElasticsearchStorage(client)

	err := s.TestConnection(context.Background())
	require.Error(t, err)
}

func TestGetClassifiedByID_Success(t *testing.T) {
	t.Helper()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"hits": map[string]any{
				"hits": []map[string]any{
					{
						"_index": "cbc_classified_content",
						"_id":    "doc-1",
						"_source": map[string]any{
							"id":            "",
							"url":           "https://cbc.ca/article",
							"source_name":   "cbc",
							"title":         "Classified Article",
							"content_type":  "article",
							"quality_score": 80,
						},
					},
				},
			},
		})
	}

	client := newTestESClient(t, handler)
	s := NewElasticsearchStorage(client)

	content, err := s.GetClassifiedByID(context.Background(), "doc-1")
	require.NoError(t, err)
	assert.Equal(t, "doc-1", content.ID) // Filled from _id
	assert.Equal(t, "Classified Article", content.Title)
}

func TestGetClassifiedByID_NotFound(t *testing.T) {
	t.Helper()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"hits": map[string]any{
				"hits": []any{},
			},
		})
	}

	client := newTestESClient(t, handler)
	s := NewElasticsearchStorage(client)

	_, err := s.GetClassifiedByID(context.Background(), "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "classified document not found")
}

func TestGetRawContentByID_Success(t *testing.T) {
	t.Helper()

	handler := func(w http.ResponseWriter, r *http.Request) {
		// Verify the correct index is queried
		assert.Contains(t, r.URL.Path, "cbc_raw_content")

		writeJSON(t, w, map[string]any{
			"hits": map[string]any{
				"hits": []map[string]any{
					{
						"_index": "cbc_raw_content",
						"_id":    "doc-1",
						"_source": map[string]any{
							"id":          "doc-1",
							"url":         "https://cbc.ca/article",
							"source_name": "cbc",
							"title":       "Raw Article",
						},
					},
				},
			},
		})
	}

	client := newTestESClient(t, handler)
	s := NewElasticsearchStorage(client)

	content, err := s.GetRawContentByID(context.Background(), "doc-1", "cbc")
	require.NoError(t, err)
	assert.Equal(t, "doc-1", content.ID)
	assert.Equal(t, "Raw Article", content.Title)
}

func TestGetRawContentByID_NotFound(t *testing.T) {
	t.Helper()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"hits": map[string]any{
				"hits": []any{},
			},
		})
	}

	client := newTestESClient(t, handler)
	s := NewElasticsearchStorage(client)

	_, err := s.GetRawContentByID(context.Background(), "nonexistent", "cbc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "raw content not found")
}

func TestIsCrimeSubcategory(t *testing.T) {
	t.Helper()

	crimeTopics := []string{
		"violent_crime", "property_crime", "drug_crime",
		"organized_crime", "criminal_justice",
	}
	for _, topic := range crimeTopics {
		assert.True(t, isCrimeSubcategory(topic), "expected %q to be a crime subcategory", topic)
	}

	notCrime := []string{"local_news", "technology", "crime", "sports", ""}
	for _, topic := range notCrime {
		assert.False(t, isCrimeSubcategory(topic), "expected %q to NOT be a crime subcategory", topic)
	}
}

func TestNewComponentLogger(t *testing.T) {
	t.Helper()

	logger, err := NewComponentLogger("test-component")
	require.NoError(t, err)
	assert.NotNil(t, logger)
}
