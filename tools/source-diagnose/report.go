package main

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
)

const (
	maxTitleDisplay = 40
	tabMinWidth     = 0
	tabWidth        = 4
	tabPadding      = 2
)

// DocStats holds extracted statistics for a single document.
type DocStats struct {
	URL           string  `json:"url"`
	Title         string  `json:"title"`
	WordCount     int     `json:"word_count"`
	ContentLength int     `json:"content_length"`
	IndexedAt     string  `json:"indexed_at"`
	QualityScore  float64 `json:"quality_score"`
}

// computeStats extracts statistics from Elasticsearch hit documents.
func computeStats(docs []map[string]any) []DocStats {
	stats := make([]DocStats, 0, len(docs))

	for _, doc := range docs {
		stat := DocStats{
			URL:       getString(doc, "url"),
			Title:     getString(doc, "title"),
			IndexedAt: getString(doc, "indexed_at"),
		}

		content := getString(doc, "content")
		stat.WordCount = countWords(content)
		stat.ContentLength = len(content)
		stat.QualityScore = getFloat(doc, "quality_score")

		stats = append(stats, stat)
	}

	return stats
}

// writeTable writes a formatted text table of document stats.
func writeTable(w io.Writer, stats []DocStats) {
	tw := tabwriter.NewWriter(w, tabMinWidth, tabWidth, tabPadding, ' ', 0)

	fmt.Fprintln(tw, "URL\tTITLE\tWORDS\tLENGTH\tQUALITY\tINDEXED_AT")
	fmt.Fprintln(tw, "---\t-----\t-----\t------\t-------\t----------")

	for i := range stats {
		title := truncateTitle(stats[i].Title)
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%.2f\t%s\n",
			stats[i].URL,
			title,
			stats[i].WordCount,
			stats[i].ContentLength,
			stats[i].QualityScore,
			stats[i].IndexedAt,
		)
	}

	tw.Flush()
}

// writeJSON writes document stats as JSON.
func writeJSON(w io.Writer, stats []DocStats) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(stats); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}

	return nil
}

// truncateTitle shortens a title for display.
func truncateTitle(title string) string {
	if len(title) <= maxTitleDisplay {
		return title
	}

	return title[:maxTitleDisplay-3] + "..."
}

// getString safely extracts a string value from a map.
func getString(m map[string]any, key string) string {
	val, ok := m[key]
	if !ok {
		return ""
	}

	str, ok := val.(string)
	if !ok {
		return ""
	}

	return str
}

// getFloat safely extracts a float64 value from a map.
func getFloat(m map[string]any, key string) float64 {
	val, ok := m[key]
	if !ok {
		return 0
	}

	f, ok := val.(float64)
	if !ok {
		return 0
	}

	return f
}
