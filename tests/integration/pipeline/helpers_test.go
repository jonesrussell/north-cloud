//go:build integration

package pipeline_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

const (
	authURL          = "http://localhost:8040"
	sourceManagerURL = "http://localhost:8050"
	crawlerURL       = "http://localhost:8060"
	publisherURL     = "http://localhost:8070"
	classifierURL    = "http://localhost:8071"
	indexManagerURL   = "http://localhost:8090"
	esURL            = "http://localhost:9200"

	healthTimeout    = 2 * time.Minute
	healthInterval   = 2 * time.Second
	pipelineTimeout  = 3 * time.Minute
	pollInterval     = 5 * time.Second

	testUsername = "testuser"
	testPassword = "testpass"
)

// waitForHealth polls a service health endpoint until it responds 200 or times out.
func waitForHealth(t *testing.T, name, url string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), healthTimeout)
	defer cancel()

	healthURL := url + "/health"
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("service %s did not become healthy at %s within %v", name, healthURL, healthTimeout)
		default:
		}

		resp, err := http.Get(healthURL) //nolint:gosec // test-only URL
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Logf("service %s is healthy", name)
				return
			}
		}
		time.Sleep(healthInterval)
	}
}

// getAuthToken authenticates and returns a JWT token.
func getAuthToken(t *testing.T) string {
	t.Helper()

	body := map[string]string{
		"username": testUsername,
		"password": testPassword,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal auth body: %v", err)
	}

	resp, err := http.Post(authURL+"/api/v1/auth/login", "application/json", bytes.NewReader(jsonBody)) //nolint:gosec
	if err != nil {
		t.Fatalf("auth request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("auth returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
		t.Fatalf("decode auth response: %v", decodeErr)
	}

	token, ok := result["token"].(string)
	if !ok {
		t.Fatal("auth response missing token field")
	}

	return token
}

// authedRequest creates an HTTP request with the JWT token.
func authedRequest(t *testing.T, method, url string, body any, token string) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, doErr := http.DefaultClient.Do(req)
	if doErr != nil {
		t.Fatalf("%s %s failed: %v", method, url, doErr)
	}

	return resp
}

// esSearch queries Elasticsearch directly and returns hits.
func esSearch(t *testing.T, index string) []map[string]any {
	t.Helper()

	query := map[string]any{
		"query": map[string]any{"match_all": map[string]any{}},
	}

	queryJSON, err := json.Marshal(query)
	if err != nil {
		t.Fatalf("marshal ES query: %v", err)
	}

	searchURL := fmt.Sprintf("%s/%s/_search", esURL, index)
	resp, postErr := http.Post(searchURL, "application/json", bytes.NewReader(queryJSON)) //nolint:gosec
	if postErr != nil {
		t.Fatalf("ES search failed: %v", postErr)
	}
	defer resp.Body.Close()

	var result struct {
		Hits struct {
			Hits []struct {
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
		t.Fatalf("decode ES response: %v", decodeErr)
	}

	docs := make([]map[string]any, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		docs = append(docs, hit.Source)
	}

	return docs
}

// pollForDocs polls an ES index until at least minCount documents appear.
func pollForDocs(t *testing.T, index string, minCount int) []map[string]any {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), pipelineTimeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %d documents in %s", minCount, index)
		default:
		}

		docs := esSearch(t, index)
		if len(docs) >= minCount {
			return docs
		}

		t.Logf("waiting for documents in %s (have %d, want %d)", index, len(docs), minCount)
		time.Sleep(pollInterval)
	}
}
