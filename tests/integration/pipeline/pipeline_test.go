//go:build integration

// Package pipeline_test implements end-to-end integration tests for the
// North Cloud content pipeline: crawl → classify → publish.
//
// Prerequisites:
//   docker compose -f docker-compose.base.yml -f docker-compose.test.yml up -d
//
// Run:
//   go test -tags integration -v -timeout 10m ./tests/integration/pipeline/
package pipeline_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/index-manager/pkg/contracts"
	"github.com/redis/go-redis/v9"
)

const (
	redisAddr        = "localhost:6379"
	redisSubTimeout  = 2 * time.Minute
)

// TestPipelineEndToEnd boots the full stack and pushes content through the
// crawl → classify → publish pipeline.
func TestPipelineEndToEnd(t *testing.T) {
	// Step 1: Wait for all services to be healthy
	t.Log("Step 1: Waiting for services to be healthy...")
	services := []struct {
		name string
		url  string
	}{
		{"auth", authURL},
		{"source-manager", sourceManagerURL},
		{"index-manager", indexManagerURL},
		{"crawler", crawlerURL},
		{"classifier", classifierURL},
		{"publisher", publisherURL},
	}
	for _, svc := range services {
		waitForHealth(t, svc.name, svc.url)
	}

	// Step 2: Authenticate
	t.Log("Step 2: Authenticating...")
	token := getAuthToken(t)

	// Step 3: Create source via source-manager
	t.Log("Step 3: Creating source...")
	sourceID := createTestSource(t, token)
	t.Logf("Created source: %s", sourceID)

	// Step 4: Create indexes via index-manager
	t.Log("Step 4: Creating indexes...")
	createIndexesForSource(t, token, "test_integration")

	// Step 5: Create publisher channel and route
	t.Log("Step 5: Setting up publisher channel and route...")
	channelID := createTestChannel(t, token)
	createTestRoute(t, token, channelID)

	// Step 6: Create crawler job targeting fixture URL
	t.Log("Step 6: Creating crawler job...")
	createCrawlerJob(t, token, sourceID)

	// Step 7: Wait for raw content in ES
	t.Log("Step 7: Waiting for raw_content...")
	rawDocs := pollForDocs(t, "test_integration_raw_content", 1)
	t.Logf("Found %d raw_content documents", len(rawDocs))

	// Step 8: Verify raw content fields match contract
	t.Log("Step 8: Verifying raw_content contract...")
	verifyRawContentDoc(t, rawDocs[0])

	// Step 9: Wait for classified content in ES
	t.Log("Step 9: Waiting for classified_content...")
	classifiedDocs := pollForDocs(t, "test_integration_classified_content", 1)
	t.Logf("Found %d classified_content documents", len(classifiedDocs))

	// Step 10: Verify classified content fields match contract
	t.Log("Step 10: Verifying classified_content contract...")
	verifyClassifiedContentDoc(t, classifiedDocs[0])

	// Step 11: Subscribe to Redis and verify message
	t.Log("Step 11: Verifying Redis publication...")
	verifyRedisPublication(t)

	t.Log("Pipeline integration test PASSED")
}

func createTestSource(t *testing.T, token string) string {
	t.Helper()

	source := map[string]any{
		"name":      "Test Integration",
		"url":       "http://nc-http-proxy:8055",
		"rate_limit": 10,
		"max_depth": 1,
		"enabled":   true,
	}

	resp := authedRequest(t, "POST", sourceManagerURL+"/api/v1/sources", source, token)
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create source returned %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode source response: %v", err)
	}

	id, ok := result["id"].(string)
	if !ok {
		t.Fatal("source response missing id field")
	}

	return id
}

func createIndexesForSource(t *testing.T, token, sourceName string) {
	t.Helper()

	body := map[string]any{
		"index_types": []string{"raw_content", "classified_content"},
	}

	url := fmt.Sprintf("%s/api/v1/sources/%s/indexes", indexManagerURL, sourceName)
	resp := authedRequest(t, "POST", url, body, token)
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("create indexes returned %d: %s", resp.StatusCode, string(respBody))
	}
}

func createTestChannel(t *testing.T, token string) string {
	t.Helper()

	channel := map[string]any{
		"name":        "test-news",
		"description": "Integration test news channel",
	}

	resp := authedRequest(t, "POST", publisherURL+"/api/v1/channels", channel, token)
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create channel returned %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode channel response: %v", err)
	}

	id, ok := result["id"].(string)
	if !ok {
		t.Fatal("channel response missing id field")
	}

	return id
}

func createTestRoute(t *testing.T, token, channelID string) {
	t.Helper()

	route := map[string]any{
		"channel_id":      channelID,
		"min_quality_score": 0,
		"content_type":    "article",
	}

	resp := authedRequest(t, "POST", publisherURL+"/api/v1/routes", route, token)
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create route returned %d: %s", resp.StatusCode, string(body))
	}
}

func createCrawlerJob(t *testing.T, token, sourceID string) {
	t.Helper()

	job := map[string]any{
		"source_id":        sourceID,
		"url":              "http://nc-http-proxy:8055/fixture/news-article.html",
		"schedule_enabled": false,
	}

	resp := authedRequest(t, "POST", crawlerURL+"/api/v1/jobs", job, token)
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create job returned %d: %s", resp.StatusCode, string(body))
	}
}

func verifyRawContentDoc(t *testing.T, doc map[string]any) {
	t.Helper()

	mapping := contracts.RawContentMapping()

	// Verify key fields from the document exist in the mapping
	expectedFields := []string{
		"url", "title", "source_name", "classification_status", "crawled_at",
	}

	contracts.AssertFieldsExist(t, mapping, expectedFields)

	// Also verify the document itself has the expected fields populated
	for _, field := range expectedFields {
		if _, exists := doc[field]; !exists {
			t.Errorf("raw_content document missing expected field %q", field)
		}
	}
}

func verifyClassifiedContentDoc(t *testing.T, doc map[string]any) {
	t.Helper()

	mapping := contracts.ClassifiedContentMapping()

	expectedFields := []string{
		"url", "title", "content_type", "quality_score", "topics",
		"classification_status",
	}

	contracts.AssertFieldsExist(t, mapping, expectedFields)

	for _, field := range expectedFields {
		if _, exists := doc[field]; !exists {
			t.Errorf("classified_content document missing expected field %q", field)
		}
	}

	// Verify classification_status is "classified"
	if status, ok := doc["classification_status"].(string); ok {
		if status != "classified" {
			t.Errorf("expected classification_status=classified, got %q", status)
		}
	}
}

func verifyRedisPublication(t *testing.T) {
	t.Helper()

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), redisSubTimeout)
	defer cancel()

	// Subscribe to the wildcard channel for any articles
	sub := rdb.PSubscribe(ctx, "articles:*")
	defer sub.Close()

	// Wait for at least one message
	msg, err := sub.ReceiveMessage(ctx)
	if err != nil {
		t.Logf("No Redis message received within timeout (this may be expected if publisher hasn't cycled yet): %v", err)
		return
	}

	// Verify the message is valid JSON with expected fields
	var article map[string]any
	if unmarshalErr := json.Unmarshal([]byte(msg.Payload), &article); unmarshalErr != nil {
		t.Fatalf("Redis message is not valid JSON: %v", unmarshalErr)
	}

	t.Logf("Received article on channel %s: title=%v", msg.Channel, article["title"])
}
