# MCP Server - Pipeline Monitoring Tools Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add 6 MCP tools for monitoring the content pipeline (crawler → classifier → publisher), exposing stats from all three services through a unified interface.

**Architecture:** MCP tools call existing service HTTP endpoints via client wrappers. ClassifierClient needs new stats methods (the service already has endpoints). Crawler and Publisher clients already have stats methods. A new `get_pipeline_health` tool aggregates data from all services.

**Tech Stack:** Go 1.24+, HTTP REST, JSON-RPC 2.0 (MCP protocol)

---

## Prerequisites

Before starting, ensure:
- `task docker:dev:up` runs all services
- Services are accessible:
  - Crawler: `curl http://localhost:8060/health`
  - Classifier: `curl http://localhost:8071/health`
  - Publisher: `curl http://localhost:8070/health`

---

## Task 1: Add Stats Types to ClassifierClient

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/client/classifier.go`

**Step 1.1: Add stats response structs**

In `classifier.go`, add after existing types:

```go
// ClassifierStats represents overall classification statistics.
type ClassifierStats struct {
	TotalClassified int                `json:"total_classified"`
	ByContentType   map[string]int     `json:"by_content_type"`
	ByTopic         map[string]int     `json:"by_topic"`
	AvgQualityScore float64            `json:"avg_quality_score"`
	Period          string             `json:"period,omitempty"`
}

// TopicStat represents a single topic's statistics.
type TopicStat struct {
	Topic string `json:"topic"`
	Count int    `json:"count"`
}

// SourceStat represents a single source's statistics.
type SourceStat struct {
	SourceName      string  `json:"source_name"`
	Count           int     `json:"count"`
	AvgQualityScore float64 `json:"avg_quality_score,omitempty"`
}
```

**Step 1.2: Run linter to verify syntax**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run ./internal/client/...`
Expected: No errors

**Step 1.3: Commit**

```bash
git add mcp-north-cloud/internal/client/classifier.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): add classifier stats types

Adds ClassifierStats, TopicStat, and SourceStat structs
for upcoming stats client methods.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Add GetStats Method to ClassifierClient

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/client/classifier.go`
- Create: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/client/classifier_test.go`

**Step 2.1: Write failing test for GetStats**

Create `classifier_test.go`:

```go
package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClassifierClient_GetStats(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/stats" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}

		// Check query param
		if r.URL.Query().Get("date") != "today" {
			t.Errorf("expected date=today, got %s", r.URL.Query().Get("date"))
		}

		resp := map[string]any{
			"total_classified": 150,
			"by_content_type":  map[string]int{"article": 120, "page": 30},
			"by_topic":         map[string]int{"crime": 45, "news": 80},
			"avg_quality_score": 72.5,
			"period":           "today",
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	authClient := NewAuthenticatedClient("", nil)
	client := NewClassifierClient(server.URL, authClient)

	stats, err := client.GetStats("today")
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.TotalClassified != 150 {
		t.Errorf("expected total_classified 150, got %d", stats.TotalClassified)
	}
	if stats.AvgQualityScore != 72.5 {
		t.Errorf("expected avg_quality_score 72.5, got %f", stats.AvgQualityScore)
	}
}
```

**Step 2.2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && go test ./internal/client/... -run TestClassifierClient_GetStats -v`
Expected: FAIL - GetStats method doesn't exist

**Step 2.3: Implement GetStats method**

In `classifier.go`, add:

```go
// GetStats returns classification statistics.
// Period can be: "today", "week", "month", "all" (empty defaults to all).
func (c *ClassifierClient) GetStats(period string) (*ClassifierStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	url := c.baseURL + "/api/v1/stats"
	if period != "" {
		url += "?date=" + period
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var stats ClassifierStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &stats, nil
}
```

**Step 2.4: Add required imports if missing**

Ensure these imports are present in `classifier.go`:

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)
```

**Step 2.5: Run test to verify it passes**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && go test ./internal/client/... -run TestClassifierClient_GetStats -v`
Expected: PASS

**Step 2.6: Run linter**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run`
Expected: No errors

**Step 2.7: Commit**

```bash
git add mcp-north-cloud/internal/client/classifier.go mcp-north-cloud/internal/client/classifier_test.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): add GetStats method to ClassifierClient

Calls GET /api/v1/stats with optional period filter.
Returns ClassifierStats with totals, content type/topic distribution,
and average quality score.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Add GetTopicStats Method to ClassifierClient

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/client/classifier.go`
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/client/classifier_test.go`

**Step 3.1: Write failing test**

Add to `classifier_test.go`:

```go
func TestClassifierClient_GetTopicStats(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/stats/topics" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := []map[string]any{
			{"topic": "crime", "count": 45},
			{"topic": "news", "count": 80},
			{"topic": "sports", "count": 25},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	authClient := NewAuthenticatedClient("", nil)
	client := NewClassifierClient(server.URL, authClient)

	topics, err := client.GetTopicStats()
	if err != nil {
		t.Fatalf("GetTopicStats failed: %v", err)
	}

	if len(topics) != 3 {
		t.Errorf("expected 3 topics, got %d", len(topics))
	}
	if topics[0].Topic != "crime" || topics[0].Count != 45 {
		t.Errorf("unexpected first topic: %+v", topics[0])
	}
}
```

**Step 3.2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && go test ./internal/client/... -run TestClassifierClient_GetTopicStats -v`
Expected: FAIL

**Step 3.3: Implement GetTopicStats**

In `classifier.go`, add:

```go
// GetTopicStats returns topic distribution across classified content.
func (c *ClassifierClient) GetTopicStats() ([]TopicStat, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/stats/topics", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var topics []TopicStat
	if err := json.NewDecoder(resp.Body).Decode(&topics); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return topics, nil
}
```

**Step 3.4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && go test ./internal/client/... -run TestClassifierClient_GetTopicStats -v`
Expected: PASS

**Step 3.5: Run linter**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run`
Expected: No errors

**Step 3.6: Commit**

```bash
git add mcp-north-cloud/internal/client/classifier.go mcp-north-cloud/internal/client/classifier_test.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): add GetTopicStats method to ClassifierClient

Calls GET /api/v1/stats/topics and returns topic distribution.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Add GetSourceStats Method to ClassifierClient

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/client/classifier.go`
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/client/classifier_test.go`

**Step 4.1: Write failing test**

Add to `classifier_test.go`:

```go
func TestClassifierClient_GetSourceStats(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/stats/sources" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := []map[string]any{
			{"source_name": "calgaryherald", "count": 100, "avg_quality_score": 75.5},
			{"source_name": "cbc", "count": 50, "avg_quality_score": 82.0},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	authClient := NewAuthenticatedClient("", nil)
	client := NewClassifierClient(server.URL, authClient)

	sources, err := client.GetSourceStats()
	if err != nil {
		t.Fatalf("GetSourceStats failed: %v", err)
	}

	if len(sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(sources))
	}
}
```

**Step 4.2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && go test ./internal/client/... -run TestClassifierClient_GetSourceStats -v`
Expected: FAIL

**Step 4.3: Implement GetSourceStats**

In `classifier.go`, add:

```go
// GetSourceStats returns source distribution across classified content.
func (c *ClassifierClient) GetSourceStats() ([]SourceStat, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/stats/sources", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var sources []SourceStat
	if err := json.NewDecoder(resp.Body).Decode(&sources); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return sources, nil
}
```

**Step 4.4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && go test ./internal/client/... -run TestClassifierClient_GetSourceStats -v`
Expected: PASS

**Step 4.5: Commit**

```bash
git add mcp-north-cloud/internal/client/classifier.go mcp-north-cloud/internal/client/classifier_test.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): add GetSourceStats method to ClassifierClient

Calls GET /api/v1/stats/sources and returns source distribution
with counts and average quality scores.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Define Pipeline Monitoring Tools

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/mcp/tools.go`

**Step 5.1: Add getPipelineMonitoringTools function**

In `tools.go`, add before the `getAllTools` function:

```go
func getPipelineMonitoringTools() []Tool {
	return []Tool{
		{
			Name:        "get_pipeline_health",
			Description: "Get aggregated health status of the entire content pipeline (crawler → classifier → publisher). Use when: Checking overall system health or diagnosing pipeline issues.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "get_classifier_stats",
			Description: "Get classification statistics including total classified, content type distribution, topic distribution, and average quality score. Use when: Monitoring classification throughput or quality.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"period": map[string]any{
						"type":        "string",
						"description": "Time period for stats: 'today', 'week', 'month', or 'all' (default: all)",
						"enum":        []string{"today", "week", "month", "all"},
					},
				},
			},
		},
		{
			Name:        "get_classifier_topic_stats",
			Description: "Get topic distribution across all classified content. Use when: Analyzing content categorization or identifying topic trends.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "get_classifier_source_stats",
			Description: "Get source distribution across classified content with counts and average quality scores per source. Use when: Comparing source quality or volume.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "get_scheduler_metrics",
			Description: "Get crawler scheduler metrics including job counts by state, success rates, and execution statistics. Use when: Monitoring crawler health or debugging job issues.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "get_recent_articles",
			Description: "Get recently published articles from the publisher. Use when: Checking what content was recently published or verifying pipeline output.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of articles to return (default: 10, max: 50)",
						"minimum":     1,
						"maximum":     50,
					},
				},
			},
		},
	}
}
```

**Step 5.2: Update getAllTools to include pipeline tools**

In `tools.go`, update `getAllTools()`:

```go
func getAllTools() []Tool {
	tools := make([]Tool, 0, toolGroupCount*estimatedToolsPerGroup)
	tools = append(tools, getWorkflowTools()...)
	tools = append(tools, getCrawlerTools()...)
	tools = append(tools, getSourceManagerTools()...)
	tools = append(tools, getPublisherTools()...)
	tools = append(tools, getSearchTools()...)
	tools = append(tools, getClassifierTools()...)
	tools = append(tools, getIndexManagerTools()...)
	tools = append(tools, getDevelopmentTools()...)
	tools = append(tools, getProxyControlTools()...)
	tools = append(tools, getPipelineMonitoringTools()...) // NEW
	return tools
}
```

**Step 5.3: Update toolGroupCount constant**

In `tools.go`, update:

```go
const (
	toolGroupCount         = 10 // Was 9, now 10 with pipeline monitoring
	estimatedToolsPerGroup = 5
)
```

**Step 5.4: Run linter**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run`
Expected: No errors

**Step 5.5: Commit**

```bash
git add mcp-north-cloud/internal/mcp/tools.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): define 6 pipeline monitoring tools

Tools: get_pipeline_health, get_classifier_stats,
get_classifier_topic_stats, get_classifier_source_stats,
get_scheduler_metrics, get_recent_articles

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Register Pipeline Tool Handlers

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/mcp/server.go`

**Step 6.1: Add tool handlers to toolHandlers map**

In `server.go`, add to the `toolHandlers` map:

```go
var toolHandlers = map[string]toolHandlerFunc{
	// ... existing handlers ...

	// Pipeline monitoring tools
	"get_pipeline_health":        (*Server).handleGetPipelineHealth,
	"get_classifier_stats":       (*Server).handleGetClassifierStats,
	"get_classifier_topic_stats": (*Server).handleGetClassifierTopicStats,
	"get_classifier_source_stats": (*Server).handleGetClassifierSourceStats,
	"get_scheduler_metrics":      (*Server).handleGetSchedulerMetrics,
	"get_recent_articles":        (*Server).handleGetRecentArticles,
}
```

**Step 6.2: Commit (handlers will be implemented next)**

```bash
git add mcp-north-cloud/internal/mcp/server.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): register pipeline monitoring tool handlers

Registers 6 handler functions for pipeline monitoring tools.
Handler implementations follow in next commit.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Implement Classifier Stats Handlers

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/mcp/handlers.go`

**Step 7.1: Add classifier stats argument struct**

In `handlers.go`, add:

```go
// Pipeline monitoring argument structs
type getClassifierStatsArgs struct {
	Period string `json:"period"`
}

type getRecentArticlesArgs struct {
	Limit int `json:"limit"`
}
```

**Step 7.2: Implement handleGetClassifierStats**

In `handlers.go`, add:

```go
// handleGetClassifierStats returns classification statistics.
func (s *Server) handleGetClassifierStats(id any, arguments json.RawMessage) *Response {
	if s.classifierClient == nil {
		return s.errorResponse(id, InvalidParams, "classifier client not configured")
	}

	var args getClassifierStatsArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
	}

	// Default to "all" if no period specified
	period := args.Period
	if period == "" {
		period = "all"
	}

	stats, err := s.classifierClient.GetStats(period)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to get classifier stats: %v", err))
	}

	return s.successResponse(id, stats)
}
```

**Step 7.3: Implement handleGetClassifierTopicStats**

In `handlers.go`, add:

```go
// handleGetClassifierTopicStats returns topic distribution.
func (s *Server) handleGetClassifierTopicStats(id any, _ json.RawMessage) *Response {
	if s.classifierClient == nil {
		return s.errorResponse(id, InvalidParams, "classifier client not configured")
	}

	topics, err := s.classifierClient.GetTopicStats()
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to get topic stats: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"topics": topics,
		"count":  len(topics),
	})
}
```

**Step 7.4: Implement handleGetClassifierSourceStats**

In `handlers.go`, add:

```go
// handleGetClassifierSourceStats returns source distribution.
func (s *Server) handleGetClassifierSourceStats(id any, _ json.RawMessage) *Response {
	if s.classifierClient == nil {
		return s.errorResponse(id, InvalidParams, "classifier client not configured")
	}

	sources, err := s.classifierClient.GetSourceStats()
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to get source stats: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"sources": sources,
		"count":   len(sources),
	})
}
```

**Step 7.5: Run linter**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run`
Expected: No errors (or errors about missing handlers - we'll add those next)

**Step 7.6: Commit**

```bash
git add mcp-north-cloud/internal/mcp/handlers.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): implement classifier stats handlers

Adds handlers for get_classifier_stats, get_classifier_topic_stats,
and get_classifier_source_stats tools.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Implement Scheduler and Publisher Handlers

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/mcp/handlers.go`

**Step 8.1: Implement handleGetSchedulerMetrics**

In `handlers.go`, add:

```go
// handleGetSchedulerMetrics returns crawler scheduler metrics.
func (s *Server) handleGetSchedulerMetrics(id any, _ json.RawMessage) *Response {
	if s.crawlerClient == nil {
		return s.errorResponse(id, InvalidParams, "crawler client not configured")
	}

	metrics, err := s.crawlerClient.GetSchedulerMetrics()
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to get scheduler metrics: %v", err))
	}

	return s.successResponse(id, metrics)
}
```

**Step 8.2: Implement handleGetRecentArticles**

In `handlers.go`, add:

```go
// handleGetRecentArticles returns recently published articles.
func (s *Server) handleGetRecentArticles(id any, arguments json.RawMessage) *Response {
	if s.publisherClient == nil {
		return s.errorResponse(id, InvalidParams, "publisher client not configured")
	}

	var args getRecentArticlesArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
	}

	// Default and cap limit
	limit := args.Limit
	if limit <= 0 {
		limit = 10
	}
	const maxLimit = 50
	if limit > maxLimit {
		limit = maxLimit
	}

	// Use GetPublishHistory with limit
	history, err := s.publisherClient.GetPublishHistory("", limit, 0)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to get recent articles: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"articles": history,
		"count":    len(history),
	})
}
```

**Step 8.3: Run linter**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run`
Expected: No errors (or error about handleGetPipelineHealth - we'll add that next)

**Step 8.4: Commit**

```bash
git add mcp-north-cloud/internal/mcp/handlers.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): implement scheduler and publisher handlers

Adds handlers for get_scheduler_metrics and get_recent_articles tools.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Implement Pipeline Health Handler

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/mcp/handlers.go`

**Step 9.1: Implement handleGetPipelineHealth**

This handler aggregates data from all three services to provide a unified health view:

```go
// PipelineHealth represents the aggregated health of all pipeline services.
type PipelineHealth struct {
	Status     string                 `json:"status"` // healthy, degraded, unhealthy
	Crawler    map[string]any         `json:"crawler"`
	Classifier map[string]any         `json:"classifier"`
	Publisher  map[string]any         `json:"publisher"`
	Errors     []string               `json:"errors,omitempty"`
}

// handleGetPipelineHealth returns aggregated pipeline health.
func (s *Server) handleGetPipelineHealth(id any, _ json.RawMessage) *Response {
	health := PipelineHealth{
		Status:     "healthy",
		Crawler:    make(map[string]any),
		Classifier: make(map[string]any),
		Publisher:  make(map[string]any),
		Errors:     make([]string, 0),
	}

	// Collect crawler metrics
	if s.crawlerClient != nil {
		metrics, err := s.crawlerClient.GetSchedulerMetrics()
		if err != nil {
			health.Errors = append(health.Errors, "crawler: "+err.Error())
			health.Status = "degraded"
		} else {
			health.Crawler["total_jobs"] = metrics.TotalJobs
			health.Crawler["running_jobs"] = metrics.RunningJobs
			health.Crawler["pending_jobs"] = metrics.PendingJobs
			health.Crawler["failed_jobs"] = metrics.FailedJobs
			health.Crawler["success_rate"] = metrics.SuccessRate
		}
	} else {
		health.Crawler["status"] = "not configured"
	}

	// Collect classifier stats
	if s.classifierClient != nil {
		stats, err := s.classifierClient.GetStats("today")
		if err != nil {
			health.Errors = append(health.Errors, "classifier: "+err.Error())
			health.Status = "degraded"
		} else {
			health.Classifier["total_classified_today"] = stats.TotalClassified
			health.Classifier["avg_quality_score"] = stats.AvgQualityScore
		}
	} else {
		health.Classifier["status"] = "not configured"
	}

	// Collect publisher stats
	if s.publisherClient != nil {
		stats, err := s.publisherClient.GetStats()
		if err != nil {
			health.Errors = append(health.Errors, "publisher: "+err.Error())
			health.Status = "degraded"
		} else {
			health.Publisher["total_published"] = stats.TotalPublished
			health.Publisher["channels"] = len(stats.ArticlesByChannel)
		}
	} else {
		health.Publisher["status"] = "not configured"
	}

	// If all services failed, mark as unhealthy
	if len(health.Errors) >= 3 {
		health.Status = "unhealthy"
	}

	return s.successResponse(id, health)
}
```

**Step 9.2: Run linter and all tests**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run && go test ./...`
Expected: All pass

**Step 9.3: Commit**

```bash
git add mcp-north-cloud/internal/mcp/handlers.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): implement get_pipeline_health handler

Aggregates health from crawler, classifier, and publisher services.
Returns unified status (healthy/degraded/unhealthy) with metrics
from each service and any errors encountered.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Add Missing Client Method (GetPublishHistory with params)

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/client/publisher.go`

**Step 10.1: Check if GetPublishHistory accepts parameters**

First, verify the current signature of `GetPublishHistory`. If it doesn't accept `channelName`, `limit`, and `offset`, update it:

```go
// GetPublishHistory returns publishing history with optional filters.
func (c *PublisherClient) GetPublishHistory(channelName string, limit, offset int) ([]PublishHistory, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	url := c.baseURL + "/api/v1/publish-history"

	// Build query params
	params := make([]string, 0, 3)
	if channelName != "" {
		params = append(params, "channel_name="+channelName)
	}
	if limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", limit))
	}
	if offset > 0 {
		params = append(params, fmt.Sprintf("offset=%d", offset))
	}
	if len(params) > 0 {
		url += "?" + strings.Join(params, "&")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var history []PublishHistory
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return history, nil
}
```

**Step 10.2: Ensure strings import is present**

Add `"strings"` to imports if not present.

**Step 10.3: Run linter and tests**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run && go test ./...`
Expected: All pass

**Step 10.4: Commit**

```bash
git add mcp-north-cloud/internal/client/publisher.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): add filter params to GetPublishHistory

Supports optional channelName, limit, and offset parameters
for filtering publish history results.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: Build and Integration Testing

**Files:**
- No file changes - verification only

**Step 11.1: Build MCP server**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && task build`
Expected: Build succeeds

**Step 11.2: Test get_pipeline_health tool**

Run:
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_pipeline_health","arguments":{}}}' | ./bin/mcp-north-cloud
```
Expected: JSON response with crawler, classifier, publisher status

**Step 11.3: Test get_classifier_stats tool**

Run:
```bash
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_classifier_stats","arguments":{"period":"today"}}}' | ./bin/mcp-north-cloud
```
Expected: JSON response with classification stats

**Step 11.4: Test get_classifier_topic_stats tool**

Run:
```bash
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_classifier_topic_stats","arguments":{}}}' | ./bin/mcp-north-cloud
```
Expected: JSON response with topic distribution

**Step 11.5: Test get_scheduler_metrics tool**

Run:
```bash
echo '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_scheduler_metrics","arguments":{}}}' | ./bin/mcp-north-cloud
```
Expected: JSON response with scheduler metrics

**Step 11.6: Test get_recent_articles tool**

Run:
```bash
echo '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_recent_articles","arguments":{"limit":5}}}' | ./bin/mcp-north-cloud
```
Expected: JSON response with recent articles

**Step 11.7: Document any issues found**

Note any failures or unexpected behavior for follow-up.

---

## Task 12: Update Documentation

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/README.md`

**Step 12.1: Add pipeline monitoring tools section**

Add a new section documenting the pipeline monitoring tools:

```markdown
## Pipeline Monitoring Tools

These tools provide visibility into the content pipeline.

| Tool | Description |
|------|-------------|
| `get_pipeline_health` | Aggregated health from all services (crawler, classifier, publisher) |
| `get_classifier_stats` | Classification statistics with optional period filter |
| `get_classifier_topic_stats` | Topic distribution across classified content |
| `get_classifier_source_stats` | Source distribution with quality scores |
| `get_scheduler_metrics` | Crawler scheduler job counts and success rates |
| `get_recent_articles` | Recently published articles (limit 1-50) |

### Example Usage

```json
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{
  "name":"get_pipeline_health",
  "arguments":{}
}}
```

Response:
```json
{
  "status": "healthy",
  "crawler": {"total_jobs": 5, "running_jobs": 1, "success_rate": 0.95},
  "classifier": {"total_classified_today": 150, "avg_quality_score": 72.5},
  "publisher": {"total_published": 1200, "channels": 3}
}
```
```

**Step 12.2: Update tool count in README**

Update any references to total tool count (now 34+ tools).

**Step 12.3: Commit**

```bash
git add mcp-north-cloud/README.md
git commit -m "$(cat <<'EOF'
docs(mcp-north-cloud): document pipeline monitoring tools

Adds documentation for 6 new pipeline monitoring tools with
example usage and response format.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Summary

This plan implements Phase 2 with 12 tasks:

| Task | Description | Files |
|------|-------------|-------|
| 1 | Add stats types to ClassifierClient | classifier.go |
| 2 | Add GetStats method | classifier.go, classifier_test.go |
| 3 | Add GetTopicStats method | classifier.go, classifier_test.go |
| 4 | Add GetSourceStats method | classifier.go, classifier_test.go |
| 5 | Define 6 pipeline monitoring tools | tools.go |
| 6 | Register tool handlers | server.go |
| 7 | Implement classifier stats handlers | handlers.go |
| 8 | Implement scheduler/publisher handlers | handlers.go |
| 9 | Implement pipeline health handler | handlers.go |
| 10 | Update GetPublishHistory with params | publisher.go |
| 11 | Integration testing | (verification only) |
| 12 | Update documentation | README.md |

**New Tools Added:**
1. `get_pipeline_health` - Aggregated health from all services
2. `get_classifier_stats` - Classification statistics
3. `get_classifier_topic_stats` - Topic distribution
4. `get_classifier_source_stats` - Source distribution
5. `get_scheduler_metrics` - Crawler scheduler health
6. `get_recent_articles` - Recently published articles

**Total: ~12 commits, following TDD pattern**
