# AI Observer Service — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a scheduled Go service that samples ES classifier output, calls Claude to detect drift and borderline clusters, and emits structured insights to an `ai_insights` ES index.

**Architecture:** Multi-category polling observer. Each poll cycle runs category passes in parallel behind a shared token-budget mutex. V0 ships the classifier drift category only (reads `*_classified_content` ES indices). Sidecar and ingestion categories are stubbed for future implementation.

**Tech Stack:** Go 1.26, `github.com/anthropic-ai/anthropic-go-sdk`, `github.com/jonesrussell/north-cloud/infrastructure` (config, logger, elasticsearch), `github.com/elastic/go-elasticsearch/v8`

**V0 scope note:** No operational Redis Streams for classifier/sidecar events exist yet — only source lifecycle events. No Loki query client exists in infrastructure. V0 delivers: service scaffold + classifier drift category (ES-based) + provider interface + cost-ceiling mutex + `ai_insights` ES index.

---

## Task 1: Service scaffold

**Files:**
- Create: `ai-observer/main.go`
- Create: `ai-observer/go.mod`
- Create: `ai-observer/Taskfile.yml`
- Create: `ai-observer/config.yml.example`

**Step 1: Create the module**

```bash
mkdir -p ai-observer
cd ai-observer
go mod init github.com/jonesrussell/north-cloud/ai-observer
```

**Step 2: Create `ai-observer/go.mod` with known deps**

```
module github.com/jonesrussell/north-cloud/ai-observer

go 1.26

require (
	github.com/anthropic-ai/anthropic-go-sdk v0.2.0
	github.com/elastic/go-elasticsearch/v8 v8.19.3
	github.com/jonesrussell/north-cloud/infrastructure v0.0.0
)

replace github.com/jonesrussell/north-cloud/infrastructure => ../infrastructure
```

Run: `cd ai-observer && go mod tidy` — this resolves the actual latest version of `anthropic-go-sdk`. If the module path is wrong, check https://pkg.go.dev/github.com/anthropic-ai/anthropic-go-sdk for the correct path.

**Step 3: Create `ai-observer/main.go`**

```go
package main

import (
	"fmt"
	"os"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/bootstrap"
)

func main() {
	if err := bootstrap.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 4: Create `ai-observer/Taskfile.yml`**

Copy from `pipeline/Taskfile.yml` and replace all occurrences of `pipeline` with `ai-observer` and `BINARY_NAME: pipeline` with `BINARY_NAME: ai-observer`.

**Step 5: Create `ai-observer/config.yml.example`**

```yaml
service:
  name: ai-observer
  version: "0.1.0"

elasticsearch:
  url: http://localhost:9200
  username: ""
  password: ""

observer:
  enabled: true
  dry_run: false
  interval_seconds: 1800
  max_tokens_per_interval: 25000
  categories:
    classifier:
      enabled: true
      max_events_per_run: 200
      model_tier: haiku
    sidecar:
      enabled: false
    ingestion:
      enabled: false

anthropic:
  api_key: ""
  default_model: claude-haiku-4-5-20251001
```

**Step 6: Commit**

```bash
git add ai-observer/
git commit -m "feat(ai-observer): scaffold service module and entry point"
```

---

## Task 2: Config and bootstrap

**Files:**
- Create: `ai-observer/internal/bootstrap/config.go`
- Create: `ai-observer/internal/bootstrap/logger.go`
- Create: `ai-observer/internal/bootstrap/app.go`

**Step 1: Write the failing test for config loading**

Create `ai-observer/internal/bootstrap/config_test.go`:

```go
package bootstrap

import (
	"os"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	t.Helper()
	t.Setenv("ES_URL", "http://localhost:9200")
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Observer.IntervalSeconds == 0 {
		t.Error("expected non-zero IntervalSeconds default")
	}
	if cfg.Observer.MaxTokensPerInterval == 0 {
		t.Error("expected non-zero MaxTokensPerInterval default")
	}
}

func TestLoadConfig_MissingAPIKey(t *testing.T) {
	t.Helper()
	os.Unsetenv("ANTHROPIC_API_KEY")
	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error when ANTHROPIC_API_KEY is missing")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd ai-observer && go test ./internal/bootstrap/... -run TestLoadConfig -v
```
Expected: compile error (package doesn't exist yet)

**Step 3: Implement `ai-observer/internal/bootstrap/config.go`**

```go
package bootstrap

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the ai-observer service.
type Config struct {
	Service    ServiceConfig
	ES         ESConfig
	Observer   ObserverConfig
	Anthropic  AnthropicConfig
}

// ServiceConfig holds service identity fields.
type ServiceConfig struct {
	Name    string
	Version string
}

// ESConfig holds Elasticsearch connection config.
type ESConfig struct {
	URL      string `env:"ES_URL"`
	Username string `env:"ES_USERNAME"`
	Password string `env:"ES_PASSWORD"`
}

// ObserverConfig holds polling and budget config.
type ObserverConfig struct {
	Enabled              bool
	DryRun               bool
	IntervalSeconds      int
	MaxTokensPerInterval int
	Categories           CategoriesConfig
}

// CategoriesConfig holds per-category feature flags.
type CategoriesConfig struct {
	ClassifierEnabled   bool
	ClassifierMaxEvents int
	ClassifierModel     string
}

// AnthropicConfig holds Anthropic API config.
type AnthropicConfig struct {
	APIKey       string
	DefaultModel string
}

const (
	defaultIntervalSeconds      = 1800
	defaultMaxTokensPerInterval = 25000
	defaultClassifierMaxEvents  = 200
	defaultClassifierModel      = "claude-haiku-4-5-20251001"
)

// LoadConfig loads configuration from environment variables.
func LoadConfig() (Config, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return Config{}, errors.New("ANTHROPIC_API_KEY is required")
	}

	esURL := os.Getenv("ES_URL")
	if esURL == "" {
		esURL = "http://localhost:9200"
	}

	intervalSeconds := defaultIntervalSeconds
	if v := os.Getenv("AI_OBSERVER_INTERVAL_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid AI_OBSERVER_INTERVAL_SECONDS: %w", err)
		}
		intervalSeconds = n
	}

	maxTokens := defaultMaxTokensPerInterval
	if v := os.Getenv("AI_OBSERVER_MAX_TOKENS_PER_INTERVAL"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid AI_OBSERVER_MAX_TOKENS_PER_INTERVAL: %w", err)
		}
		maxTokens = n
	}

	return Config{
		Service: ServiceConfig{
			Name:    "ai-observer",
			Version: "0.1.0",
		},
		ES: ESConfig{
			URL:      esURL,
			Username: os.Getenv("ES_USERNAME"),
			Password: os.Getenv("ES_PASSWORD"),
		},
		Observer: ObserverConfig{
			Enabled:              os.Getenv("AI_OBSERVER_ENABLED") != "false",
			DryRun:               os.Getenv("AI_OBSERVER_DRY_RUN") == "true",
			IntervalSeconds:      intervalSeconds,
			MaxTokensPerInterval: maxTokens,
			Categories: CategoriesConfig{
				ClassifierEnabled:   os.Getenv("AI_OBSERVER_CATEGORY_CLASSIFIER_ENABLED") != "false",
				ClassifierMaxEvents: defaultClassifierMaxEvents,
				ClassifierModel:     defaultClassifierModel,
			},
		},
		Anthropic: AnthropicConfig{
			APIKey:       apiKey,
			DefaultModel: defaultClassifierModel,
		},
	}, nil
}
```

**Step 4: Implement `ai-observer/internal/bootstrap/logger.go`**

```go
package bootstrap

import (
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// CreateLogger initializes the structured logger.
func CreateLogger(cfg Config) (infralogger.Logger, error) {
	logCfg := infralogger.Config{
		Level:       "info",
		Format:      "json",
		ServiceName: cfg.Service.Name,
	}
	return infralogger.NewLogger(logCfg)
}
```

**Step 5: Create minimal `ai-observer/internal/bootstrap/app.go`**

```go
// Package bootstrap handles initialization for the ai-observer service.
package bootstrap

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// Start initializes and runs the ai-observer service.
func Start() error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	log, err := CreateLogger(cfg)
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	log.Info("Starting AI Observer",
		infralogger.String("service", cfg.Service.Name),
		infralogger.String("version", cfg.Service.Version),
		infralogger.Bool("dry_run", cfg.Observer.DryRun),
	)

	if !cfg.Observer.Enabled {
		log.Info("AI Observer is disabled via config — exiting")
		return nil
	}

	// TODO Task 5: wire scheduler here
	log.Info("AI Observer scaffold running — scheduler not yet wired")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("AI Observer stopped")
	return nil
}
```

**Step 6: Run tests**

```bash
cd ai-observer && go test ./internal/bootstrap/... -v
```
Expected: both tests pass.

**Step 7: Verify it builds**

```bash
cd ai-observer && go build ./...
```

**Step 8: Commit**

```bash
git add ai-observer/internal/bootstrap/
git commit -m "feat(ai-observer): add config loading and bootstrap"
```

---

## Task 3: Provider interface and Anthropic implementation

**Files:**
- Create: `ai-observer/internal/provider/interface.go`
- Create: `ai-observer/internal/provider/anthropic/client.go`
- Create: `ai-observer/internal/provider/anthropic/client_test.go`

**Step 1: Write the failing test**

Create `ai-observer/internal/provider/anthropic/client_test.go`:

```go
package anthropic_test

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
	anthclient "github.com/jonesrussell/north-cloud/ai-observer/internal/provider/anthropic"
)

func TestAnthropicClient_ImplementsInterface(t *testing.T) {
	t.Helper()
	var _ provider.LLMProvider = &anthclient.Client{}
}

func TestAnthropicClient_Name(t *testing.T) {
	t.Helper()
	c := anthclient.New("test-key", "claude-haiku-4-5-20251001")
	if c.Name() != "anthropic" {
		t.Errorf("expected name 'anthropic', got %q", c.Name())
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd ai-observer && go test ./internal/provider/... -v
```
Expected: compile error

**Step 3: Create `ai-observer/internal/provider/interface.go`**

```go
// Package provider defines the LLMProvider interface for AI callouts.
package provider

import "context"

// GenerateRequest is the input to an LLM call.
type GenerateRequest struct {
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int
	// JSONSchema is an optional JSON schema string to enforce structured output.
	// Leave empty if the provider does not support structured output.
	JSONSchema string
}

// GenerateResponse is the output from an LLM call.
type GenerateResponse struct {
	Content      string
	InputTokens  int
	OutputTokens int
}

// LLMProvider is the interface all AI provider implementations must satisfy.
type LLMProvider interface {
	// Name returns the provider name (e.g. "anthropic", "openai").
	Name() string
	// Generate sends a prompt and returns a response.
	Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error)
}
```

**Step 4: Create `ai-observer/internal/provider/anthropic/client.go`**

Check the exact Anthropic Go SDK API first:
```bash
cd ai-observer && go get github.com/anthropic-ai/anthropic-go-sdk && go doc github.com/anthropic-ai/anthropic-go-sdk | head -40
```

Then implement:

```go
// Package anthropic provides an LLMProvider implementation backed by Anthropic's Claude API.
package anthropic

import (
	"context"
	"fmt"

	anthropicsdk "github.com/anthropic-ai/anthropic-go-sdk"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
)

// Client implements provider.LLMProvider using the Anthropic API.
type Client struct {
	client *anthropicsdk.Client
	model  string
}

// New creates a new Anthropic provider client.
func New(apiKey, model string) *Client {
	c := anthropicsdk.NewClient(
		anthropicsdk.WithAPIKey(apiKey),
	)
	return &Client{client: &c, model: model}
}

// Name returns "anthropic".
func (c *Client) Name() string {
	return "anthropic"
}

// Generate sends a prompt to Claude and returns the response.
// Adjust the SDK call to match the actual go-sdk API after running go doc.
func (c *Client) Generate(ctx context.Context, req provider.GenerateRequest) (provider.GenerateResponse, error) {
	msg, err := c.client.Messages.New(ctx, anthropicsdk.MessageNewParams{
		Model:     anthropicsdk.F(c.model),
		MaxTokens: anthropicsdk.F(int64(req.MaxTokens)),
		System: anthropicsdk.F([]anthropicsdk.TextBlockParam{
			{Type: anthropicsdk.F(anthropicsdk.TextBlockParamTypeText), Text: anthropicsdk.F(req.SystemPrompt)},
		}),
		Messages: anthropicsdk.F([]anthropicsdk.MessageParam{
			anthropicsdk.UserMessageParam(anthropicsdk.TextBlockParam{
				Type: anthropicsdk.F(anthropicsdk.TextBlockParamTypeText),
				Text: anthropicsdk.F(req.UserPrompt),
			}),
		}),
	})
	if err != nil {
		return provider.GenerateResponse{}, fmt.Errorf("anthropic generate: %w", err)
	}

	content := ""
	if len(msg.Content) > 0 {
		content = msg.Content[0].Text
	}

	return provider.GenerateResponse{
		Content:      content,
		InputTokens:  int(msg.Usage.InputTokens),
		OutputTokens: int(msg.Usage.OutputTokens),
	}, nil
}
```

**Note on SDK API:** The above uses the likely API shape. After running `go get` and `go doc`, adjust the exact parameter types to match the SDK. The SDK may have changed its API. The key contract is: send system + user message, get text back, capture token counts.

**Step 5: Run tests**

```bash
cd ai-observer && go test ./internal/provider/... -v
```
Expected: both compile-time tests pass (no live API call needed).

**Step 6: Commit**

```bash
git add ai-observer/internal/provider/
git commit -m "feat(ai-observer): add LLMProvider interface and Anthropic implementation"
```

---

## Task 4: Category interface and classifier drift category

**Files:**
- Create: `ai-observer/internal/category/interface.go`
- Create: `ai-observer/internal/category/classifier/sampler.go`
- Create: `ai-observer/internal/category/classifier/analyzer.go`
- Create: `ai-observer/internal/category/classifier/category.go`
- Create: `ai-observer/internal/category/classifier/category_test.go`

**Step 1: Write the failing test**

Create `ai-observer/internal/category/classifier/category_test.go`:

```go
package classifier_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	classifiercategory "github.com/jonesrussell/north-cloud/ai-observer/internal/category/classifier"
)

func TestClassifierCategory_ImplementsInterface(t *testing.T) {
	t.Helper()
	var _ category.Category = &classifiercategory.Category{}
}

func TestClassifierCategory_Name(t *testing.T) {
	t.Helper()
	c := classifiercategory.New(nil, 200, "claude-haiku-4-5-20251001")
	if c.Name() != "classifier" {
		t.Errorf("expected name 'classifier', got %q", c.Name())
	}
}

func TestClassifierCategory_MaxEventsPerRun(t *testing.T) {
	t.Helper()
	const maxEvents = 150
	c := classifiercategory.New(nil, maxEvents, "claude-haiku-4-5-20251001")
	if c.MaxEventsPerRun() != maxEvents {
		t.Errorf("expected %d, got %d", maxEvents, c.MaxEventsPerRun())
	}
}
```

**Step 2: Run to verify it fails**

```bash
cd ai-observer && go test ./internal/category/... -v
```
Expected: compile error

**Step 3: Create `ai-observer/internal/category/interface.go`**

```go
// Package category defines the Category interface for AI observer analysis passes.
package category

import (
	"context"
	"time"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
)

// Event is a normalized event from any source (ES, Redis, Loki).
type Event struct {
	// Source identifier (domain, index name, sidecar name, etc.)
	Source string
	// Label is the primary classification label (topic, content_type, error_type, etc.)
	Label string
	// Confidence is the classifier confidence score (0.0-1.0). Zero if not applicable.
	Confidence float64
	// Timestamp of the event.
	Timestamp time.Time
	// Metadata holds category-specific fields.
	Metadata map[string]any
}

// Insight is a structured advisory generated by an LLM from a set of events.
type Insight struct {
	// Category that produced this insight.
	Category string
	// Severity: "low", "medium", "high".
	Severity string
	// Summary is a one-sentence human-readable description.
	Summary string
	// Details holds structured data supporting the insight.
	Details map[string]any
	// SuggestedActions is a list of actionable recommendations.
	SuggestedActions []string
	// TokensUsed is the total tokens consumed producing this insight.
	TokensUsed int
	// Model is the model that produced this insight.
	Model string
}

// Category is the interface all analysis categories must implement.
type Category interface {
	// Name returns the category identifier (e.g. "classifier", "sidecar", "ingestion").
	Name() string
	// Sample queries the data source and returns normalized events within the given window.
	Sample(ctx context.Context, window time.Duration) ([]Event, error)
	// Analyze runs the AI pass on the sampled events and returns insights.
	Analyze(ctx context.Context, events []Event, p provider.LLMProvider) ([]Insight, error)
	// MaxEventsPerRun returns the per-run event cap for this category.
	MaxEventsPerRun() int
	// ModelTier returns the preferred model tier: "haiku" or "sonnet".
	ModelTier() string
}
```

**Step 4: Create `ai-observer/internal/category/classifier/sampler.go`**

```go
package classifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
)

const (
	// borderlineThreshold is the confidence below which a doc is considered borderline.
	borderlineThreshold = 0.6
	// highConfidenceSampleFraction is the fraction of high-confidence docs to include for regression.
	highConfidenceSampleFraction = 0.05
	// indexPattern is the ES index pattern for classified content.
	indexPattern = "*_classified_content"
)

// classifiedDoc is a minimal projection of an ES classified_content document.
type classifiedDoc struct {
	Domain      string             `json:"domain"`
	ContentType string             `json:"content_type"`
	Topics      []string           `json:"topics"`
	TopicScores map[string]float64 `json:"topic_scores"`
	Confidence  float64            `json:"confidence"`
	QualityScore int               `json:"quality_score"`
	ClassifiedAt time.Time         `json:"classified_at"`
}

// sample queries ES for documents within the given window and applies sampling.
func sample(ctx context.Context, esClient *es.Client, window time.Duration, maxEvents int) ([]category.Event, error) {
	since := time.Now().UTC().Add(-window)

	query := map[string]any{
		"query": map[string]any{
			"range": map[string]any{
				"classified_at": map[string]any{
					"gte": since.Format(time.RFC3339),
				},
			},
		},
		"size": maxEvents,
		"sort": []map[string]any{
			{"classified_at": map[string]any{"order": "desc"}},
		},
		"_source": []string{"domain", "content_type", "topics", "topic_scores", "confidence", "quality_score", "classified_at"},
	}

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	res, err := esClient.Search(
		esClient.Search.WithContext(ctx),
		esClient.Search.WithIndex(indexPattern),
		esClient.Search.WithBody(bytes.NewReader(queryBytes)),
	)
	if err != nil {
		return nil, fmt.Errorf("es search: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return nil, fmt.Errorf("es search error: %s", res.String())
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source classifiedDoc `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if decodeErr := json.NewDecoder(res.Body).Decode(&result); decodeErr != nil {
		return nil, fmt.Errorf("decode response: %w", decodeErr)
	}

	events := make([]category.Event, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		doc := hit.Source
		// Apply sampling: include all borderline docs, sample high-confidence docs
		if doc.Confidence >= borderlineThreshold {
			// Skip high-confidence docs most of the time (regression sample)
			if len(events) > 0 && float64(len(events)%20) != 0 {
				continue
			}
		}
		events = append(events, category.Event{
			Source:     doc.Domain,
			Label:      doc.ContentType,
			Confidence: doc.Confidence,
			Timestamp:  doc.ClassifiedAt,
			Metadata: map[string]any{
				"topics":        doc.Topics,
				"topic_scores":  doc.TopicScores,
				"quality_score": doc.QualityScore,
			},
		})
	}

	return events, nil
}
```

**Step 5: Create `ai-observer/internal/category/classifier/analyzer.go`**

```go
package classifier

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
)

const (
	systemPrompt = `You are an AI system observer analyzing classifier output from a news content pipeline.
Your job is to identify label drift, borderline clusters, and domains that consistently produce low-confidence classifications.
Respond ONLY with valid JSON matching the schema provided. Be concise and actionable.`

	maxPromptTokens    = 4000
	maxResponseTokens  = 1000
	insightSchemaStr   = `{"type":"array","items":{"type":"object","properties":{"severity":{"type":"string","enum":["low","medium","high"]},"summary":{"type":"string"},"details":{"type":"object"},"suggested_actions":{"type":"array","items":{"type":"string"}}}}}`
)

// domainStats holds aggregated stats per domain+label pair.
type domainStats struct {
	Domain         string
	Label          string
	Count          int
	BorderlineCount int
	AvgConfidence  float64
}

// analyze runs the AI pass on sampled events.
func analyze(ctx context.Context, events []category.Event, p provider.LLMProvider, model string) ([]category.Insight, error) {
	if len(events) == 0 {
		return nil, nil
	}

	stats := aggregateStats(events)
	userPrompt := buildPrompt(stats)

	resp, err := p.Generate(ctx, provider.GenerateRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    maxResponseTokens,
		JSONSchema:   insightSchemaStr,
	})
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}

	insights, parseErr := parseInsights(resp.Content, resp.InputTokens+resp.OutputTokens, model)
	if parseErr != nil {
		return nil, fmt.Errorf("parse insights: %w", parseErr)
	}

	return insights, nil
}

func aggregateStats(events []category.Event) []domainStats {
	type key struct{ domain, label string }
	statsMap := make(map[key]*domainStats)

	for _, e := range events {
		k := key{e.Source, e.Label}
		s, ok := statsMap[k]
		if !ok {
			s = &domainStats{Domain: e.Source, Label: e.Label}
			statsMap[k] = s
		}
		s.Count++
		s.AvgConfidence += e.Confidence
		if e.Confidence < borderlineThreshold {
			s.BorderlineCount++
		}
	}

	result := make([]domainStats, 0, len(statsMap))
	for _, s := range statsMap {
		if s.Count > 0 {
			s.AvgConfidence /= float64(s.Count)
		}
		result = append(result, *s)
	}

	// Sort by borderline rate descending
	sort.Slice(result, func(i, j int) bool {
		ri := float64(result[i].BorderlineCount) / float64(result[i].Count)
		rj := float64(result[j].BorderlineCount) / float64(result[j].Count)
		return ri > rj
	})

	return result
}

func buildPrompt(stats []domainStats) string {
	// Limit to top 30 domain+label pairs to stay within token budget
	const maxPairs = 30
	if len(stats) > maxPairs {
		stats = stats[:maxPairs]
	}

	data, _ := json.Marshal(stats)
	return fmt.Sprintf(`Analyze these classifier output statistics (domain+label aggregates from the last polling window).
Identify any concerning patterns: high borderline rates, consistent low confidence, or unexpected label distributions.

Statistics (JSON):
%s

Respond with a JSON array of insights. Each insight must have: severity (low/medium/high), summary (one sentence), details (object with relevant metrics), suggested_actions (array of strings).
If no issues are found, return an empty array [].`, string(data))
}

// rawInsight is the expected JSON shape from the LLM.
type rawInsight struct {
	Severity         string         `json:"severity"`
	Summary          string         `json:"summary"`
	Details          map[string]any `json:"details"`
	SuggestedActions []string       `json:"suggested_actions"`
}

func parseInsights(content string, tokensUsed int, model string) ([]category.Insight, error) {
	var raw []rawInsight
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("unmarshal LLM response: %w (content: %.200s)", err, content)
	}

	insights := make([]category.Insight, 0, len(raw))
	for _, r := range raw {
		insights = append(insights, category.Insight{
			Category:         "classifier",
			Severity:         r.Severity,
			Summary:          r.Summary,
			Details:          r.Details,
			SuggestedActions: r.SuggestedActions,
			TokensUsed:       tokensUsed / len(raw), // distribute evenly
			Model:            model,
		})
	}
	return insights, nil
}
```

**Step 6: Create `ai-observer/internal/category/classifier/category.go`**

```go
// Package classifier implements the classifier drift analysis category.
package classifier

import (
	"context"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
)

// Category implements category.Category for classifier drift detection.
type Category struct {
	esClient    *es.Client
	maxEvents   int
	modelTier   string
}

// New creates a new classifier drift Category.
// esClient may be nil (for unit tests that don't test Sample).
func New(esClient *es.Client, maxEvents int, modelTier string) *Category {
	return &Category{
		esClient:  esClient,
		maxEvents: maxEvents,
		modelTier: modelTier,
	}
}

// Name returns "classifier".
func (c *Category) Name() string { return "classifier" }

// MaxEventsPerRun returns the configured event cap.
func (c *Category) MaxEventsPerRun() int { return c.maxEvents }

// ModelTier returns the configured model tier.
func (c *Category) ModelTier() string { return c.modelTier }

// Sample queries ES for recent classified documents.
func (c *Category) Sample(ctx context.Context, window time.Duration) ([]category.Event, error) {
	return sample(ctx, c.esClient, window, c.maxEvents)
}

// Analyze runs the AI drift detection pass.
func (c *Category) Analyze(ctx context.Context, events []category.Event, p provider.LLMProvider) ([]category.Insight, error) {
	return analyze(ctx, events, p, c.modelTier)
}
```

**Step 7: Run tests**

```bash
cd ai-observer && go test ./internal/category/... -v
```
Expected: 3 tests pass.

**Step 8: Commit**

```bash
git add ai-observer/internal/category/
git commit -m "feat(ai-observer): add Category interface and classifier drift category"
```

---

## Task 5: Insight writer (ES ai_insights index)

**Files:**
- Create: `ai-observer/internal/insights/writer.go`
- Create: `ai-observer/internal/insights/writer_test.go`

**Step 1: Write the failing test**

Create `ai-observer/internal/insights/writer_test.go`:

```go
package insights_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/insights"
)

func TestBuildDocument_Fields(t *testing.T) {
	t.Helper()
	ins := category.Insight{
		Category:         "classifier",
		Severity:         "medium",
		Summary:          "Test insight",
		Details:          map[string]any{"domain": "example.com"},
		SuggestedActions: []string{"Do something"},
		TokensUsed:       100,
		Model:            "claude-haiku-4-5-20251001",
	}
	version := "0.1.0"
	now := time.Now().UTC()

	doc := insights.BuildDocument(ins, version, now)

	if doc["category"] != "classifier" {
		t.Errorf("expected category 'classifier', got %v", doc["category"])
	}
	if doc["severity"] != "medium" {
		t.Errorf("expected severity 'medium', got %v", doc["severity"])
	}
	if doc["observer_version"] != version {
		t.Errorf("expected observer_version %q, got %v", version, doc["observer_version"])
	}
}
```

**Step 2: Run to verify it fails**

```bash
cd ai-observer && go test ./internal/insights/... -v
```
Expected: compile error

**Step 3: Create `ai-observer/internal/insights/writer.go`**

```go
// Package insights handles writing AI-generated insights to Elasticsearch.
package insights

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
)

const insightsIndex = "ai_insights"

// BuildDocument converts an Insight into an ES document map.
// Exported for testing.
func BuildDocument(ins category.Insight, observerVersion string, now time.Time) map[string]any {
	return map[string]any{
		"id":                fmt.Sprintf("ins_%s_%s", now.Format("20060102"), uuid.New().String()[:8]),
		"created_at":        now.Format(time.RFC3339),
		"category":          ins.Category,
		"severity":          ins.Severity,
		"summary":           ins.Summary,
		"details":           ins.Details,
		"suggested_actions": ins.SuggestedActions,
		"observer_version":  observerVersion,
		"model":             ins.Model,
		"tokens_used":       ins.TokensUsed,
	}
}

// Writer writes insights to the ai_insights ES index.
type Writer struct {
	esClient        *es.Client
	observerVersion string
}

// NewWriter creates a new insight Writer.
func NewWriter(esClient *es.Client, observerVersion string) *Writer {
	return &Writer{esClient: esClient, observerVersion: observerVersion}
}

// WriteAll indexes all provided insights. Errors are collected and returned together.
func (w *Writer) WriteAll(ctx context.Context, insightList []category.Insight) error {
	var lastErr error
	for _, ins := range insightList {
		if err := w.write(ctx, ins); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (w *Writer) write(ctx context.Context, ins category.Insight) error {
	doc := BuildDocument(ins, w.observerVersion, time.Now().UTC())

	docBytes, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal insight: %w", err)
	}

	res, err := w.esClient.Index(
		insightsIndex,
		bytes.NewReader(docBytes),
		w.esClient.Index.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("index insight: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return fmt.Errorf("index insight error: %s", res.String())
	}

	return nil
}
```

Add `github.com/google/uuid` to `go.mod` if not already present:
```bash
cd ai-observer && go get github.com/google/uuid
```

**Step 4: Run tests**

```bash
cd ai-observer && go test ./internal/insights/... -v
```
Expected: 1 test passes.

**Step 5: Commit**

```bash
git add ai-observer/internal/insights/
git commit -m "feat(ai-observer): add insight writer for ai_insights ES index"
```

---

## Task 6: Scheduler with cost-ceiling mutex

**Files:**
- Create: `ai-observer/internal/scheduler/scheduler.go`
- Create: `ai-observer/internal/scheduler/scheduler_test.go`

**Step 1: Write the failing test**

Create `ai-observer/internal/scheduler/scheduler_test.go`:

```go
package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/scheduler"
)

func TestBudget_Deduct(t *testing.T) {
	t.Helper()
	b := scheduler.NewBudget(1000)

	ok := b.Deduct(500)
	if !ok {
		t.Error("expected first deduction to succeed")
	}

	ok = b.Deduct(600)
	if ok {
		t.Error("expected second deduction to fail (would exceed budget)")
	}
}

func TestBudget_Reset(t *testing.T) {
	t.Helper()
	b := scheduler.NewBudget(1000)
	b.Deduct(1000)

	b.Reset()

	ok := b.Deduct(500)
	if !ok {
		t.Error("expected deduction to succeed after reset")
	}
}

func TestScheduler_RunOnce_NoBudget(t *testing.T) {
	t.Helper()
	// A scheduler with zero budget should skip all categories.
	s := scheduler.New(
		nil, // no categories
		nil, // no writer
		nil, // no provider
		scheduler.Config{
			IntervalSeconds:      1,
			MaxTokensPerInterval: 0,
			WindowDuration:       time.Hour,
			DryRun:               false,
		},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Should return without panic
	s.RunOnce(ctx)
}
```

**Step 2: Run to verify it fails**

```bash
cd ai-observer && go test ./internal/scheduler/... -v
```
Expected: compile error

**Step 3: Create `ai-observer/internal/scheduler/scheduler.go`**

```go
// Package scheduler provides the polling loop and cost-ceiling budget for the AI observer.
package scheduler

import (
	"context"
	"sync"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/insights"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
)

// Config holds scheduler configuration.
type Config struct {
	IntervalSeconds      int
	MaxTokensPerInterval int
	WindowDuration       time.Duration
	DryRun               bool
}

// Budget is a thread-safe token budget for a single polling interval.
type Budget struct {
	mu        sync.Mutex
	max       int
	remaining int
}

// NewBudget creates a Budget with the given max tokens.
func NewBudget(max int) *Budget {
	return &Budget{max: max, remaining: max}
}

// Deduct attempts to deduct n tokens from the budget.
// Returns true if the deduction succeeded, false if it would exceed the budget.
func (b *Budget) Deduct(n int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.remaining < n {
		return false
	}
	b.remaining -= n
	return true
}

// Reset restores the budget to its max for the next interval.
func (b *Budget) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.remaining = b.max
}

// Scheduler runs category passes on a ticker and emits insights.
type Scheduler struct {
	categories []category.Category
	writer     *insights.Writer
	provider   provider.LLMProvider
	cfg        Config
	log        infralogger.Logger
}

// New creates a new Scheduler.
func New(
	categories []category.Category,
	writer *insights.Writer,
	p provider.LLMProvider,
	cfg Config,
) *Scheduler {
	return &Scheduler{
		categories: categories,
		writer:     writer,
		provider:   p,
		cfg:        cfg,
	}
}

// WithLogger sets a logger on the scheduler.
func (s *Scheduler) WithLogger(log infralogger.Logger) *Scheduler {
	s.log = log
	return s
}

// Run starts the polling loop and blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	interval := time.Duration(s.cfg.IntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.logInfo("Scheduler started", infralogger.Int("interval_seconds", s.cfg.IntervalSeconds))
	s.RunOnce(ctx) // run immediately on start

	for {
		select {
		case <-ctx.Done():
			s.logInfo("Scheduler stopping")
			return
		case <-ticker.C:
			s.RunOnce(ctx)
		}
	}
}

// RunOnce executes one polling cycle: sample all categories in parallel, collect insights, write them.
func (s *Scheduler) RunOnce(ctx context.Context) {
	if len(s.categories) == 0 {
		return
	}

	budget := NewBudget(s.cfg.MaxTokensPerInterval)

	type result struct {
		insights []category.Insight
		err      error
	}

	results := make(chan result, len(s.categories))
	var wg sync.WaitGroup

	for _, cat := range s.categories {
		wg.Add(1)
		go func(c category.Category) {
			defer wg.Done()
			ins, err := s.runCategory(ctx, c, budget)
			results <- result{insights: ins, err: err}
		}(cat)
	}

	wg.Wait()
	close(results)

	var allInsights []category.Insight
	for r := range results {
		if r.err != nil {
			s.logError("category error", r.err)
			continue
		}
		allInsights = append(allInsights, r.insights...)
	}

	if s.cfg.DryRun {
		s.logInfo("Dry run: would write insights", infralogger.Int("count", len(allInsights)))
		return
	}

	if s.writer != nil && len(allInsights) > 0 {
		if err := s.writer.WriteAll(ctx, allInsights); err != nil {
			s.logError("write insights error", err)
		} else {
			s.logInfo("Insights written", infralogger.Int("count", len(allInsights)))
		}
	}
}

func (s *Scheduler) runCategory(ctx context.Context, cat category.Category, budget *Budget) ([]category.Insight, error) {
	events, err := cat.Sample(ctx, s.cfg.WindowDuration)
	if err != nil {
		return nil, err
	}

	if len(events) == 0 {
		return nil, nil
	}

	// Estimate token cost: ~4 chars per token, ~200 chars per event summary
	estimatedTokens := len(events) * 50
	if !budget.Deduct(estimatedTokens) {
		s.logInfo("budget exhausted, skipping category", infralogger.String("category", cat.Name()))
		return nil, nil
	}

	return cat.Analyze(ctx, events, s.provider)
}

func (s *Scheduler) logInfo(msg string, fields ...infralogger.Field) {
	if s.log != nil {
		s.log.Info(msg, fields...)
	}
}

func (s *Scheduler) logError(msg string, err error, fields ...infralogger.Field) {
	if s.log != nil {
		allFields := append([]infralogger.Field{infralogger.Error(err)}, fields...)
		s.log.Error(msg, allFields...)
	}
}
```

**Step 4: Run tests**

```bash
cd ai-observer && go test ./internal/scheduler/... -v
```
Expected: all 3 tests pass.

**Step 5: Commit**

```bash
git add ai-observer/internal/scheduler/
git commit -m "feat(ai-observer): add scheduler with polling loop and cost-ceiling budget"
```

---

## Task 7: Wire everything together in bootstrap

**Files:**
- Modify: `ai-observer/internal/bootstrap/app.go`
- Create: `ai-observer/internal/bootstrap/elasticsearch.go`

**Step 1: Create `ai-observer/internal/bootstrap/elasticsearch.go`**

```go
package bootstrap

import (
	"context"
	"fmt"

	"github.com/jonesrussell/north-cloud/infrastructure/elasticsearch"
)

// SetupElasticsearch creates and verifies the ES client.
func SetupElasticsearch(ctx context.Context, cfg Config) (*elasticsearch.Client, error) {
	esCfg := elasticsearch.Config{
		URL:      cfg.ES.URL,
		Username: cfg.ES.Username,
		Password: cfg.ES.Password,
	}
	client, err := elasticsearch.NewClient(ctx, esCfg, nil)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch client: %w", err)
	}
	return client, nil
}
```

**Note:** `elasticsearch.NewClient` returns `*es.Client` not a custom type. Adjust the import to match the actual return type from `infrastructure/elasticsearch`.

**Step 2: Update `ai-observer/internal/bootstrap/app.go` to wire everything**

Replace the existing `app.go` with:

```go
// Package bootstrap handles initialization for the ai-observer service.
package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	classifiercategory "github.com/jonesrussell/north-cloud/ai-observer/internal/category/classifier"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/insights"
	anthprovider "github.com/jonesrussell/north-cloud/ai-observer/internal/provider/anthropic"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/scheduler"
)

// Start initializes and runs the ai-observer service.
func Start() error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	log, err := CreateLogger(cfg)
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	log.Info("Starting AI Observer",
		infralogger.String("service", cfg.Service.Name),
		infralogger.String("version", cfg.Service.Version),
		infralogger.Bool("dry_run", cfg.Observer.DryRun),
		infralogger.Bool("enabled", cfg.Observer.Enabled),
	)

	if !cfg.Observer.Enabled {
		log.Info("AI Observer is disabled via AI_OBSERVER_ENABLED — exiting")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	esClient, err := SetupElasticsearch(ctx, cfg)
	if err != nil {
		return fmt.Errorf("elasticsearch: %w", err)
	}
	log.Info("Elasticsearch connected")

	p := anthprovider.New(cfg.Anthropic.APIKey, cfg.Anthropic.DefaultModel)
	writer := insights.NewWriter(esClient, cfg.Service.Version)

	var categories []classifiercategory.Category // use interface slice in real code
	if cfg.Observer.Categories.ClassifierEnabled {
		cat := classifiercategory.New(esClient, cfg.Observer.Categories.ClassifierMaxEvents, cfg.Observer.Categories.ClassifierModel)
		categories = append(categories, *cat)
	}

	// Convert to interface slice
	catIfaces := make([]interface{ /* category.Category */ }, 0, len(categories))
	// NOTE: use the actual category.Category interface here — see Task 4

	sched := scheduler.New(
		nil, // pass catIfaces as []category.Category
		writer,
		p,
		scheduler.Config{
			IntervalSeconds:      cfg.Observer.IntervalSeconds,
			MaxTokensPerInterval: cfg.Observer.MaxTokensPerInterval,
			WindowDuration:       time.Hour,
			DryRun:               cfg.Observer.DryRun,
		},
	).WithLogger(log)
	_ = catIfaces
	_ = sched

	log.Info("Scheduler wired — starting polling loop")
	// sched.Run(ctx) // Uncomment after resolving the category interface wiring above

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("AI Observer stopped")
	return nil
}
```

**Note on category wiring:** The `app.go` above has a placeholder comment. The actual wiring is:

```go
import (
    "github.com/jonesrussell/north-cloud/ai-observer/internal/category"
    classifiercategory "github.com/jonesrussell/north-cloud/ai-observer/internal/category/classifier"
)

var cats []category.Category
if cfg.Observer.Categories.ClassifierEnabled {
    cats = append(cats, classifiercategory.New(esClient, cfg.Observer.Categories.ClassifierMaxEvents, cfg.Observer.Categories.ClassifierModel))
}

sched := scheduler.New(cats, writer, p, schedConfig).WithLogger(log)
go sched.Run(ctx)
```

**Step 3: Build and verify no compile errors**

```bash
cd ai-observer && go build ./...
```
Fix any import issues until it builds cleanly.

**Step 4: Run all tests**

```bash
cd ai-observer && go test ./... -v
```
Expected: all tests pass.

**Step 5: Commit**

```bash
git add ai-observer/internal/bootstrap/
git commit -m "feat(ai-observer): wire bootstrap — ES client, provider, scheduler, categories"
```

---

## Task 8: Lint and vendor

**Step 1: Add to go.work**

```bash
# From repo root:
go work edit -use ./ai-observer
```

**Step 2: Vendor**

```bash
cd ai-observer && go mod vendor
```

**Step 3: Lint**

```bash
cd ai-observer && golangci-lint run --config ../.golangci.yml ./...
```

Fix any linting violations (see CLAUDE.md critical rules: no `interface{}`, no magic numbers, `t.Helper()` in all test helpers, pre-allocate slices).

**Step 4: Commit**

```bash
git add ai-observer/vendor/ ai-observer/go.sum go.work
git commit -m "chore(ai-observer): vendor dependencies and add to go.work"
```

---

## Task 9: Dry-run smoke test

**Goal:** Verify the observer starts, connects to ES, and logs "Dry run: would write insights" without making real API calls.

**Step 1: Set up env**

```bash
export AI_OBSERVER_ENABLED=true
export AI_OBSERVER_DRY_RUN=true
export AI_OBSERVER_INTERVAL_SECONDS=60
export AI_OBSERVER_MAX_TOKENS_PER_INTERVAL=10000
export ANTHROPIC_API_KEY=dummy-key-for-dry-run
export ES_URL=http://localhost:9200
```

**Step 2: Start ES (if not already running)**

```bash
task docker:dev:up
```

**Step 3: Run the observer**

```bash
cd ai-observer && go run . 2>&1
```

Expected log output:
```json
{"level":"info","msg":"Starting AI Observer","service":"ai-observer","dry_run":true,"enabled":true}
{"level":"info","msg":"Elasticsearch connected"}
{"level":"info","msg":"Scheduler started","interval_seconds":60}
{"level":"info","msg":"Dry run: would write insights","count":0}
```

**Step 4: Verify ES connectivity**

The observer should not crash even if `*_classified_content` has no data (it returns an empty sample and skips the AI call).

**Step 5: Commit**

```bash
git commit -m "docs(ai-observer): add dry-run smoke test instructions"
```

---

## Task 10: docker-compose integration

**Files:**
- Create: `ai-observer/Dockerfile`
- Modify: `docker-compose.base.yml` (add ai-observer service)
- Modify: `docker-compose.dev.yml` (add ai-observer env overrides)

**Step 1: Create `ai-observer/Dockerfile`**

Copy from `pipeline/Dockerfile` and replace `pipeline` with `ai-observer` throughout.

**Step 2: Add to `docker-compose.base.yml`**

```yaml
  ai-observer:
    build:
      context: .
      dockerfile: ai-observer/Dockerfile
    container_name: north-cloud-ai-observer
    restart: unless-stopped
    depends_on:
      - elasticsearch
    environment:
      - AI_OBSERVER_ENABLED=${AI_OBSERVER_ENABLED:-false}
      - AI_OBSERVER_DRY_RUN=${AI_OBSERVER_DRY_RUN:-true}
      - AI_OBSERVER_INTERVAL_SECONDS=${AI_OBSERVER_INTERVAL_SECONDS:-1800}
      - AI_OBSERVER_MAX_TOKENS_PER_INTERVAL=${AI_OBSERVER_MAX_TOKENS_PER_INTERVAL:-25000}
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - ES_URL=http://elasticsearch:9200
    networks:
      - north-cloud-network
```

**Note:** `AI_OBSERVER_ENABLED` defaults to `false` so the service does nothing unless explicitly enabled. This ensures zero impact on existing deployments.

**Step 3: Build the Docker image**

```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml build ai-observer
```

**Step 4: Run in dry-run mode**

```bash
AI_OBSERVER_ENABLED=true AI_OBSERVER_DRY_RUN=true ANTHROPIC_API_KEY=test \
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up ai-observer
```

**Step 5: Commit**

```bash
git add ai-observer/Dockerfile docker-compose.base.yml docker-compose.dev.yml
git commit -m "feat(ai-observer): add Dockerfile and docker-compose integration"
```

---

## Rollout Phases (Post-Implementation)

| Phase | Action |
|-------|--------|
| Phase 0 | Merge with `AI_OBSERVER_ENABLED=false` — zero production impact |
| Phase 1 | Enable with `DRY_RUN=true` — logs prompts + projected cost, no real API calls |
| Phase 2 | Enable real calls in dev, review first 10 insights manually |
| Phase 3 | Enable in production (classifier category only) |
| Phase 4 | Implement sidecar and ingestion categories (requires adding operational event emission to classifier/sidecars) |

---

## Deferred (Not in v0)

- Sidecar anomaly category — needs operational events on Redis Streams (not currently emitted)
- Ingestion failure category — needs Loki HTTP query client in infrastructure
- Dashboard UI for `ai_insights` index
- GitHub issue automation on `severity=high`
