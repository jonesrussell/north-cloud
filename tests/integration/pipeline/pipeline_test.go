//go:build integration

// Package pipeline_test provides an end-to-end smoke test for the
// crawl -> classify -> publish pipeline.
//
// This test verifies that one article flows from a fixture URL through
// the full pipeline (Crawler -> Elasticsearch raw_content -> Classifier ->
// Elasticsearch classified_content -> Publisher -> Redis Pub/Sub).
//
// It asserts contract-relevant fields at each stage per the testing
// checklist (docs/TESTING_PIPELINE_CHECKLIST.md). It does NOT assert
// exact quality scores, exact topic lists, volume, performance, full
// schema equality, auth/security, or resilience. Production health is
// judged by the Intelligence dashboard, pipeline events, and alerts;
// this test is not a substitute.
package pipeline_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test fixture and source constants.
const (
	fixtureSourceName = "fixture_news_site_com"
	fixtureSourceURL  = "https://fixture-news-site.com"

	rawContentIndex        = fixtureSourceName + "_raw_content"
	classifiedContentIndex = fixtureSourceName + "_classified_content"
)

// Publisher channel constants for the Layer 2 integration-test channel.
const (
	testChannelName  = "Integration Test"
	testChannelSlug  = "integration-test"
	testRedisChannel = "articles:integration-test"
)

// Timing constants for delays and timeouts.
const (
	subscriberSetupDelay = 1 * time.Second
	redisMessageTimeout  = 180 * time.Second
)

// HTTP status constants.
const (
	httpStatusCreated = http.StatusCreated
)

// TestPipelineSmoke runs the full crawl -> classify -> publish pipeline
// against the fixture news site and verifies contract fields at each stage.
func TestPipelineSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping pipeline integration test in short mode")
	}

	// Step 1: Wait for all services to be healthy.
	waitForAllServices(t)

	// Step 2: Get auth token.
	token := getAuthToken(t)
	t.Log("Authenticated successfully")

	// Step 3: Create source via source-manager.
	sourceID := createSource(t, token)
	t.Logf("Created source with ID: %s", sourceID)

	// Step 4: Create a Layer 2 publisher channel that matches all articles.
	channelID := createPublisherChannel(t, token)
	t.Logf("Created publisher channel with ID: %s", channelID)

	// Step 5: Start Redis subscriber BEFORE triggering crawl (in goroutine).
	redisCh := make(chan map[string]any, 1)
	go func() {
		msg := subscribeRedis(t, testRedisChannel, redisMessageTimeout)
		redisCh <- msg
	}()

	// Step 6: Small delay to ensure subscriber is connected.
	time.Sleep(subscriberSetupDelay)

	// Step 7: Create one-time crawler job.
	createCrawlerJob(t, token, sourceID)
	t.Log("Crawler job created")

	// Step 8: Poll ES for raw content.
	rawDocID, rawSource := pollRawContent(t)
	t.Logf("Raw content document found: %s", rawDocID)

	// Step 9: Poll ES for classified content.
	classifiedDocID, classifiedSource := pollClassifiedContent(t)
	t.Logf("Classified content document found: %s", classifiedDocID)

	// Step 10: Wait for Redis message from goroutine.
	var redisMsg map[string]any
	select {
	case redisMsg = <-redisCh:
		t.Log("Redis message received on channel: " + testRedisChannel)
	case <-time.After(redisMessageTimeout):
		t.Fatal("Timed out waiting for Redis message")
	}

	// Step 11: Assert contract fields on raw content.
	assertRawContentContract(t, rawSource)

	// Step 12: Assert contract fields on classified content.
	assertClassifiedContentContract(t, classifiedSource)

	// Step 13: Assert contract fields on Redis message.
	assertRedisMessageContract(t, redisMsg)

	// Step 14: Assert end-to-end URL consistency.
	assertEndToEndURLMatch(t, classifiedSource, redisMsg)
}

// waitForAllServices polls health endpoints for all pipeline services.
func waitForAllServices(t *testing.T) {
	t.Helper()

	waitForHealth(t, "auth", authURL+"/health")
	waitForHealth(t, "source-manager", sourceManagerURL+"/health")
	waitForHealth(t, "crawler", crawlerURL+"/health")
	waitForHealth(t, "classifier", classifierURL+"/health")
	waitForHealth(t, "publisher", publisherURL+"/health")
	waitForHealth(t, "index-manager", indexManagerURL+"/health")
	waitForHealth(t, "elasticsearch", esURL+"/_cluster/health")
}

// createSource registers the fixture news site in source-manager and returns its ID.
func createSource(t *testing.T, token string) string {
	t.Helper()

	body := fmt.Sprintf(`{
		"name": %q,
		"url": %q,
		"enabled": true,
		"selectors": {
			"article": {
				"title": "h1",
				"body": "article"
			}
		}
	}`, fixtureSourceName, fixtureSourceURL)

	status, respBody := doAuthed(t, http.MethodPost, sourceManagerURL+"/api/v1/sources", token, body)
	require.Equal(t, httpStatusCreated, status,
		"create source failed: %s", string(respBody))

	var result map[string]any
	err := json.Unmarshal(respBody, &result)
	require.NoError(t, err, "unmarshal create source response")

	id, ok := result["id"].(string)
	require.True(t, ok, "source response must contain string 'id' field")
	require.NotEmpty(t, id, "source id must not be empty")

	return id
}

// createPublisherChannel creates a Layer 2 custom channel that matches all articles.
func createPublisherChannel(t *testing.T, token string) string {
	t.Helper()

	body := fmt.Sprintf(`{
		"name": %q,
		"slug": %q,
		"redis_channel": %q,
		"description": "Integration test channel - matches all articles",
		"rules": {
			"min_quality_score": 0,
			"content_types": ["article"]
		},
		"enabled": true
	}`, testChannelName, testChannelSlug, testRedisChannel)

	status, respBody := doAuthed(t, http.MethodPost, publisherURL+"/api/v1/channels", token, body)
	require.Equal(t, httpStatusCreated, status,
		"create publisher channel failed: %s", string(respBody))

	var result map[string]any
	err := json.Unmarshal(respBody, &result)
	require.NoError(t, err, "unmarshal create channel response")

	id, ok := result["id"].(string)
	require.True(t, ok, "channel response must contain string 'id' field")
	require.NotEmpty(t, id, "channel id must not be empty")

	return id
}

// createCrawlerJob creates a one-time crawler job for the fixture source.
func createCrawlerJob(t *testing.T, token, sourceID string) {
	t.Helper()

	body := fmt.Sprintf(`{
		"source_id": %q,
		"source_name": %q,
		"url": %q,
		"schedule_enabled": false
	}`, sourceID, fixtureSourceName, fixtureSourceURL)

	status, respBody := doAuthed(t, http.MethodPost, crawlerURL+"/api/v1/jobs", token, body)
	require.Equal(t, httpStatusCreated, status,
		"create crawler job failed: %s", string(respBody))
}

// pollRawContent polls Elasticsearch for a document in the raw content index.
func pollRawContent(t *testing.T) (string, map[string]any) {
	t.Helper()

	query := `{"query": {"match_all": {}}}`
	return pollES(t, rawContentIndex, query)
}

// pollClassifiedContent polls Elasticsearch for a document in the classified content index.
func pollClassifiedContent(t *testing.T) (string, map[string]any) {
	t.Helper()

	query := `{"query": {"match_all": {}}}`
	return pollES(t, classifiedContentIndex, query)
}

// assertRawContentContract asserts that the raw content document has the
// required classification_status field.
func assertRawContentContract(t *testing.T, source map[string]any) {
	t.Helper()

	assert.Contains(t, source, "classification_status",
		"raw content document must have 'classification_status' field")
}

// assertClassifiedContentContract asserts that the classified content
// document has all required contract fields.
func assertClassifiedContentContract(t *testing.T, source map[string]any) {
	t.Helper()

	assert.Contains(t, source, "content_type",
		"classified doc must have 'content_type'")
	assert.Contains(t, source, "quality_score",
		"classified doc must have 'quality_score'")
	assert.Contains(t, source, "topics",
		"classified doc must have 'topics'")
	assert.Contains(t, source, "title",
		"classified doc must have 'title'")
	assertDocHasURL(t, source)
}

// assertDocHasURL checks that the classified document has a URL field.
// The field may be 'url' or 'canonical_url' depending on the classifier output.
func assertDocHasURL(t *testing.T, source map[string]any) {
	t.Helper()

	_, hasURL := source["url"]
	_, hasCanonicalURL := source["canonical_url"]
	assert.True(t, hasURL || hasCanonicalURL,
		"classified doc must have 'url' or 'canonical_url'")
}

// assertRedisMessageContract asserts that the Redis message has all required
// contract fields per REDIS_MESSAGE_FORMAT.md.
func assertRedisMessageContract(t *testing.T, msg map[string]any) {
	t.Helper()

	assert.Contains(t, msg, "id", "Redis message must have 'id'")
	assert.Contains(t, msg, "title", "Redis message must have 'title'")
	assert.Contains(t, msg, "content_type", "Redis message must have 'content_type'")
	assert.Contains(t, msg, "quality_score", "Redis message must have 'quality_score'")
	assert.Contains(t, msg, "topics", "Redis message must have 'topics'")

	// Assert publisher metadata sub-object.
	publisherObj, ok := msg["publisher"].(map[string]any)
	require.True(t, ok, "Redis message must have 'publisher' object")

	assert.Contains(t, publisherObj, "channel",
		"publisher object must have 'channel'")
	assert.Contains(t, publisherObj, "published_at",
		"publisher object must have 'published_at'")
}

// assertEndToEndURLMatch verifies that the URL from the classified content
// document matches the URL in the Redis message, confirming the same article
// flowed through the entire pipeline.
func assertEndToEndURLMatch(t *testing.T, classifiedSource, redisMsg map[string]any) {
	t.Helper()

	classifiedURL := extractURL(classifiedSource)
	redisURL := extractURL(redisMsg)

	require.NotEmpty(t, classifiedURL, "classified document must have a URL")
	require.NotEmpty(t, redisURL, "Redis message must have a URL")
	assert.Equal(t, classifiedURL, redisURL,
		"URL in classified_content must match URL in Redis message")
}

// urlFieldNames lists the possible URL field names in pipeline documents.
var urlFieldNames = []string{"canonical_url", "url"}

// extractURL extracts a URL from a document, checking common field names.
func extractURL(doc map[string]any) string {
	for _, field := range urlFieldNames {
		if val, ok := doc[field].(string); ok && val != "" {
			return val
		}
	}
	return ""
}
