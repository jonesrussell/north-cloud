package enricher

import (
	"fmt"
	"math"
	"strings"

	"github.com/jonesrussell/north-cloud/enrichment/internal/api"
)

func confidence(request api.EnrichmentRequest, hits []Hit) float64 {
	if len(hits) == 0 {
		return emptyConfidence
	}

	bestScore := math.Min(hits[0].Score/maxSearchScore, 1)
	evidenceScore := math.Min(float64(len(hits))*evidenceWeight, evidenceWeight*3)
	score := baseConfidence + bestScore*scoreWeight + evidenceScore

	if request.Domain != "" && anyHitContains(hits, request.Domain) {
		score += domainMatchBoost
	}
	if request.Sector != "" && anyHitContains(hits, request.Sector) {
		score += sectorMatchBoost
	}
	if score > highConfidenceThreshold {
		score = highConfidenceThreshold
	}

	return math.Round(score*100) / 100
}

func anyHitContains(hits []Hit, needle string) bool {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return false
	}

	for _, hit := range hits {
		for _, value := range hit.Source {
			if strings.Contains(strings.ToLower(toString(value)), needle) {
				return true
			}
		}
	}
	return false
}

func evidenceItems(hits []Hit) []map[string]any {
	items := make([]map[string]any, 0, len(hits))
	for _, hit := range hits {
		item := map[string]any{
			"id":    hit.ID,
			"score": hit.Score,
		}
		for _, field := range []string{"title", "url", "source", "published_at"} {
			if value, ok := hit.Source[field]; ok {
				item[field] = value
			}
		}
		items = append(items, item)
	}
	return items
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}
