// Package insights handles writing AI-generated insights to Elasticsearch.
package insights

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
)

const (
	insightsIndex = "ai_insights"
	idDateFormat  = "20060102"
	idSuffixBytes = 4 // 4 random bytes → 8 hex chars.
)

// BuildDocument converts an Insight into an ES document map.
// Exported for testing.
func BuildDocument(ins category.Insight, observerVersion string, now time.Time) map[string]any {
	id := buildID(now)
	return map[string]any{
		"id":                id,
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

// buildID generates a unique insight ID of the form ins_YYYYMMDD_<8 hex chars>.
func buildID(now time.Time) string {
	buf := make([]byte, idSuffixBytes)
	if _, err := rand.Read(buf); err != nil {
		// Fallback: use nanosecond timestamp hex if crypto/rand fails.
		return fmt.Sprintf("ins_%s_%x", now.Format(idDateFormat), now.UnixNano())
	}
	return fmt.Sprintf("ins_%s_%s", now.Format(idDateFormat), hex.EncodeToString(buf))
}

// Writer writes insights to the ai_insights ES index.
type Writer struct {
	esClient        *es.Client
	observerVersion string
	dedup           *Deduplicator
}

// NewWriter creates a new insight Writer.
func NewWriter(esClient *es.Client, observerVersion string) *Writer {
	return &Writer{esClient: esClient, observerVersion: observerVersion}
}

// WithDedup attaches a deduplicator to filter repeated insights before writing.
func (w *Writer) WithDedup(d *Deduplicator) *Writer {
	w.dedup = d
	return w
}

// WriteAll indexes all provided insights. When a deduplicator is configured,
// insights with duplicate summaries within the cooldown window are skipped.
// Each remaining insight is indexed independently; all errors are joined and
// returned together.
func (w *Writer) WriteAll(ctx context.Context, insightList []category.Insight) error {
	if w.dedup != nil {
		filtered, filterErr := w.dedup.Filter(ctx, insightList)
		if filterErr == nil {
			insightList = filtered
		}
		// Dedup failure is non-fatal — proceed with unfiltered list to avoid
		// dropping insights during transient ES issues.
	}

	var errs []error
	for _, ins := range insightList {
		if err := w.write(ctx, ins); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
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
