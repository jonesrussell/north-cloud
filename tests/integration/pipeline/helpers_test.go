//go:build integration

package pipeline_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// Service URLs for the integration test environment.
const (
	authURL          = "http://localhost:8040"
	sourceManagerURL = "http://localhost:8050"
	crawlerURL       = "http://localhost:8060"
	classifierURL    = "http://localhost:8071"
	publisherURL     = "http://localhost:8070"
	indexManagerURL  = "http://localhost:8090"
	esURL            = "http://localhost:9200"
	redisAddr        = "localhost:6379"
)

// Timeouts and intervals for polling operations.
const (
	healthTimeout  = 120 * time.Second
	healthInterval = 2 * time.Second
	pollTimeout    = 120 * time.Second
	pollInterval   = 3 * time.Second
)

// esSearchResult represents the structure of an Elasticsearch search response.
type esSearchResult struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			ID     string         `json:"_id"`
			Source map[string]any `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// waitForHealth polls the given URL until it returns HTTP 200 or the timeout expires.
func waitForHealth(t *testing.T, name, url string) {
	t.Helper()

	deadline := time.Now().Add(healthTimeout)
	client := &http.Client{Timeout: healthInterval}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Logf("%s is healthy at %s", name, url)
				return
			}
		}
		time.Sleep(healthInterval)
	}

	t.Fatalf("%s did not become healthy at %s within %s", name, url, healthTimeout)
}

// loginRequest is the JSON body sent to the auth login endpoint.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// loginResponse is the JSON body returned from the auth login endpoint.
type loginResponse struct {
	Token string `json:"token"`
}

// getAuthToken authenticates against the auth service and returns a JWT token.
func getAuthToken(t *testing.T) string {
	t.Helper()

	creds := loginRequest{
		Username: "admin",
		Password: "testpass123",
	}

	body, err := json.Marshal(creds)
	require.NoError(t, err, "marshal login request")

	resp, err := http.Post(
		authURL+"/api/v1/auth/login",
		"application/json",
		strings.NewReader(string(body)),
	)
	require.NoError(t, err, "POST login")
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "read login response")
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"login failed: %s", string(respBody))

	var result loginResponse
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err, "unmarshal login response")
	require.NotEmpty(t, result.Token, "auth token must not be empty")

	return result.Token
}

// authedRequest creates an HTTP request with the Authorization bearer header set.
func authedRequest(t *testing.T, method, url, token string, body io.Reader) *http.Request {
	t.Helper()

	req, err := http.NewRequest(method, url, body)
	require.NoError(t, err, "create request %s %s", method, url)

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	return req
}

// doAuthed performs an authenticated HTTP request and returns the status code and response body.
func doAuthed(t *testing.T, method, url, token string, body string) (int, []byte) {
	t.Helper()

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req := authedRequest(t, method, url, token, bodyReader)

	client := &http.Client{Timeout: pollTimeout}
	resp, err := client.Do(req)
	require.NoError(t, err, "execute request %s %s", method, url)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "read response body from %s %s", method, url)

	return resp.StatusCode, respBody
}

// pollES polls Elasticsearch with the given query until at least one document matches
// or the timeout expires. Returns the document ID and source of the first hit.
func pollES(t *testing.T, index, queryJSON string) (string, map[string]any) {
	t.Helper()

	deadline := time.Now().Add(pollTimeout)
	searchURL := esURL + "/" + index + "/_search"
	client := &http.Client{Timeout: pollInterval}

	for time.Now().Before(deadline) {
		docID, source, found := tryESSearch(t, client, searchURL, queryJSON)
		if found {
			return docID, source
		}
		time.Sleep(pollInterval)
	}

	t.Fatalf("no documents matched in ES index %q within %s", index, pollTimeout)
	return "", nil // unreachable
}

// tryESSearch performs a single Elasticsearch search and returns results if any documents match.
func tryESSearch(t *testing.T, client *http.Client, searchURL, queryJSON string) (string, map[string]any, bool) {
	t.Helper()

	resp, err := client.Post(searchURL, "application/json", strings.NewReader(queryJSON))
	if err != nil {
		return "", nil, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, false
	}

	var result esSearchResult
	if unmarshalErr := json.Unmarshal(body, &result); unmarshalErr != nil {
		return "", nil, false
	}

	if result.Hits.Total.Value > 0 && len(result.Hits.Hits) > 0 {
		hit := result.Hits.Hits[0]
		return hit.ID, hit.Source, true
	}

	return "", nil, false
}

// subscribeRedis subscribes to a Redis Pub/Sub channel and waits for the first message.
// Returns the parsed JSON message as a map.
func subscribeRedis(t *testing.T, channel string, timeout time.Duration) map[string]any {
	t.Helper()

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	sub := rdb.Subscribe(ctx, channel)
	defer sub.Close()

	// Wait for subscription confirmation.
	_, err := sub.Receive(ctx)
	require.NoError(t, err, "subscribe to Redis channel %s", channel)

	msgCh := sub.Channel()

	select {
	case msg := <-msgCh:
		var parsed map[string]any
		unmarshalErr := json.Unmarshal([]byte(msg.Payload), &parsed)
		require.NoError(t, unmarshalErr, "unmarshal Redis message from channel %s", channel)
		return parsed
	case <-ctx.Done():
		t.Fatalf("no message received on Redis channel %q within %s", channel, timeout)
		return nil // unreachable
	}
}
