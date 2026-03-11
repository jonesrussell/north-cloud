package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/tabwriter"

	"github.com/PuerkitoBio/goquery"
)

const (
	fullPercent = 100.0
)

// CompareResult holds the comparison between ES content and live page content.
type CompareResult struct {
	URL            string  `json:"url"`
	OriginalWords  int     `json:"original_words"`
	ExtractedWords int     `json:"extracted_words"`
	LossPercent    float64 `json:"loss_percent"`
}

// comparePage fetches a live page, extracts body text, and compares word count
// with the stored ES content.
func comparePage(pageURL string, storedWords int) (CompareResult, error) {
	resp, err := http.Get(pageURL) //nolint:gosec // URL comes from ES data
	if err != nil {
		return CompareResult{}, fmt.Errorf("fetching page %s: %w", pageURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return CompareResult{}, fmt.Errorf(
			"page %s returned status %d", pageURL, resp.StatusCode,
		)
	}

	doc, parseErr := goquery.NewDocumentFromReader(resp.Body)
	if parseErr != nil {
		return CompareResult{}, fmt.Errorf("parsing page %s: %w", pageURL, parseErr)
	}

	bodyText := extractBodyText(doc)
	extractedWords := countWords(bodyText)

	return CompareResult{
		URL:            pageURL,
		OriginalWords:  storedWords,
		ExtractedWords: extractedWords,
		LossPercent:    extractionLoss(extractedWords, storedWords),
	}, nil
}

// extractBodyText extracts visible text content from an HTML document body.
func extractBodyText(doc *goquery.Document) string {
	doc.Find("script, style, noscript").Remove()

	return strings.TrimSpace(doc.Find("body").Text())
}

// extractionLoss calculates the percentage of words lost during content extraction.
// A positive value means ES has fewer words than the live page.
func extractionLoss(liveWords, storedWords int) float64 {
	if liveWords == 0 {
		return 0
	}

	return fullPercent - (float64(storedWords)/float64(liveWords))*fullPercent
}

// writeCompareResults writes comparison results as a formatted table.
func writeCompareResults(w io.Writer, results []CompareResult) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "=== Live Page Comparison ===")

	tw := tabwriter.NewWriter(w, tabMinWidth, tabWidth, tabPadding, ' ', 0)

	fmt.Fprintln(tw, "URL\tLIVE_WORDS\tSTORED_WORDS\tLOSS%")
	fmt.Fprintln(tw, "---\t----------\t------------\t-----")

	for i := range results {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%.1f%%\n",
			results[i].URL,
			results[i].ExtractedWords,
			results[i].OriginalWords,
			results[i].LossPercent,
		)
	}

	tw.Flush()
}
