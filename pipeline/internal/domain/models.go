// Package domain contains the core domain models for the pipeline observability service.
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
	"time"
)

// Stage represents a named step in the content pipeline.
type Stage string

const (
	// StageCrawled indicates content has been fetched by the crawler.
	StageCrawled Stage = "crawled"
	// StageIndexed indicates content has been stored in Elasticsearch.
	StageIndexed Stage = "indexed"
	// StageClassified indicates content has been classified (quality, topics, crime).
	StageClassified Stage = "classified"
	// StageRouted indicates content has been matched to publishing routes.
	StageRouted Stage = "routed"
	// StagePublished indicates content has been published to Redis Pub/Sub.
	StagePublished Stage = "published"
)

// validStages maps every recognised Stage value to true for O(1) lookup.
var validStages = map[Stage]bool{
	StageCrawled:    true,
	StageIndexed:    true,
	StageClassified: true,
	StageRouted:     true,
	StagePublished:  true,
}

// stageCount is the number of valid pipeline stages (used for pre-allocation).
const stageCount = 5

// AllStages returns all valid pipeline stages in order.
func AllStages() []Stage {
	stages := make([]Stage, 0, stageCount)
	stages = append(stages, StageCrawled, StageIndexed, StageClassified, StageRouted, StagePublished)
	return stages
}

// IsValid reports whether s is a recognised pipeline stage.
func (s Stage) IsValid() bool {
	return validStages[s]
}

// urlHashShortLen is the number of hex characters returned by URLHashShort.
const urlHashShortLen = 8

// unknownDomain is the fallback value when a URL cannot be parsed.
const unknownDomain = "unknown"

// wwwPrefix is the subdomain prefix stripped by ExtractDomain.
const wwwPrefix = "www."

// idempotencyKeySeparator is the delimiter used in GenerateIdempotencyKey.
const idempotencyKeySeparator = ":"

// Article represents a unique piece of content tracked across the pipeline.
type Article struct {
	URL         string    `json:"url"`
	URLHash     string    `json:"url_hash"`
	Domain      string    `json:"domain"`
	SourceName  string    `json:"source_name"`
	FirstSeenAt time.Time `json:"first_seen_at"`
}

// PipelineEvent records a single stage transition for an article.
type PipelineEvent struct {
	ID                    int64          `json:"id"`
	ArticleURL            string         `json:"article_url"`
	Stage                 Stage          `json:"stage"`
	OccurredAt            time.Time      `json:"occurred_at"`
	ReceivedAt            time.Time      `json:"received_at"`
	ServiceName           string         `json:"service_name"`
	Metadata              map[string]any `json:"metadata,omitempty"`
	MetadataSchemaVersion int            `json:"metadata_schema_version"`
	IdempotencyKey        string         `json:"idempotency_key"`
}

// IngestRequest is the payload accepted by the event ingestion endpoint.
type IngestRequest struct {
	ArticleURL     string         `binding:"required"        json:"article_url"`
	SourceName     string         `binding:"required"        json:"source_name"`
	Stage          Stage          `binding:"required"        json:"stage"`
	OccurredAt     time.Time      `binding:"required"        json:"occurred_at"`
	ServiceName    string         `binding:"required"        json:"service_name"`
	IdempotencyKey string         `binding:"required"        json:"idempotency_key"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// BatchIngestRequest wraps multiple IngestRequest items for bulk ingestion.
type BatchIngestRequest struct {
	Events []IngestRequest `binding:"required,min=1" json:"events"`
}

// FunnelStage holds aggregated counts for a single pipeline stage in a funnel report.
type FunnelStage struct {
	Name           string `json:"name"`
	Count          int64  `json:"count"`
	UniqueArticles int64  `json:"unique_articles"`
}

// FunnelResponse is the top-level response for the pipeline funnel endpoint.
type FunnelResponse struct {
	Period      string        `json:"period"`
	Timezone    string        `json:"timezone"`
	From        time.Time     `json:"from"`
	To          time.Time     `json:"to"`
	Stages      []FunnelStage `json:"stages"`
	GeneratedAt time.Time     `json:"generated_at"`
}

// URLHash returns the full SHA-256 hex digest of rawURL.
func URLHash(rawURL string) string {
	h := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(h[:])
}

// URLHashShort returns the first urlHashShortLen hex characters of the SHA-256 hash.
func URLHashShort(rawURL string) string {
	return URLHash(rawURL)[:urlHashShortLen]
}

// ExtractDomain parses rawURL and returns the hostname with any "www." prefix removed.
// On parse failure it returns "unknown".
func ExtractDomain(rawURL string) string {
	parsed, parseErr := url.Parse(rawURL)
	if parseErr != nil || parsed.Host == "" {
		return unknownDomain
	}

	host := parsed.Hostname()
	return strings.TrimPrefix(host, wwwPrefix)
}

// GenerateIdempotencyKey produces a deterministic key for deduplicating pipeline events.
// Format: {serviceName}:{stage}:{urlhash8}:{occurredAt RFC3339}.
func GenerateIdempotencyKey(serviceName string, stage Stage, articleURL string, occurredAt time.Time) string {
	return strings.Join([]string{
		serviceName,
		string(stage),
		URLHashShort(articleURL),
		occurredAt.Format(time.RFC3339),
	}, idempotencyKeySeparator)
}
