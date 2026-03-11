// Package main provides a diagnostic CLI tool for analyzing source content quality
// by querying Elasticsearch and optionally comparing with live page fetches.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	esURL := flag.String("es-url", "http://localhost:9200", "Elasticsearch URL")
	source := flag.String("source", "", "Source name to diagnose (required)")
	index := flag.String("index", "*_classified_content", "Elasticsearch index pattern")
	limit := flag.Int("limit", defaultDocLimit, "Number of documents to fetch")
	compare := flag.Bool("compare", false, "Enable live page comparison")
	jsonOut := flag.Bool("json", false, "Output in JSON format")

	flag.Parse()

	if *source == "" {
		flag.Usage()
		return fmt.Errorf("missing required flag: -source")
	}

	docs, err := fetchDocuments(*esURL, *index, *source, *limit)
	if err != nil {
		return fmt.Errorf("fetching documents: %w", err)
	}

	stats := computeStats(docs)

	if *jsonOut {
		if writeErr := writeJSON(os.Stdout, stats); writeErr != nil {
			return fmt.Errorf("writing JSON output: %w", writeErr)
		}
	} else {
		writeTable(os.Stdout, stats)
	}

	if *compare {
		results := make([]CompareResult, 0, len(stats))

		for i := range stats {
			result, compareErr := comparePage(stats[i].URL, stats[i].WordCount)
			if compareErr != nil {
				fmt.Fprintf(os.Stderr, "warn: compare failed for %s: %v\n", stats[i].URL, compareErr)

				continue
			}

			results = append(results, result)
		}

		writeCompareResults(os.Stdout, results)
	}

	return nil
}
