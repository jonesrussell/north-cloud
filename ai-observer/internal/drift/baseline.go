package drift

import (
	"time"
)

// HistogramBins is the number of bins for confidence score histograms.
const HistogramBins = 10

// ClassifiedDoc is a projection of an ES classified_content document for drift analysis.
type ClassifiedDoc struct {
	SourceRegion     string    `json:"source_region"`
	Topics           []string  `json:"topics"`
	Confidence       float64   `json:"confidence"`
	CrimeConfidence  float64   `json:"crime.final_confidence"`
	MiningConfidence float64   `json:"mining.final_confidence"`
	ClassifiedAt     time.Time `json:"classified_at"`
}

// BuildCategoryDistribution computes topic probability distributions per region and globally.
func BuildCategoryDistribution(docs []ClassifiedDoc) map[string]map[string]float64 {
	counts := make(map[string]map[string]int)
	totals := make(map[string]int)

	for _, doc := range docs {
		for _, topic := range doc.Topics {
			addCount(counts, "global", topic)
			totals["global"]++
			if doc.SourceRegion != "" {
				addCount(counts, doc.SourceRegion, topic)
				totals[doc.SourceRegion]++
			}
		}
	}

	dist := make(map[string]map[string]float64, len(counts))
	for scope, topicCounts := range counts {
		dist[scope] = make(map[string]float64, len(topicCounts))
		total := totals[scope]
		if total == 0 {
			continue
		}
		for topic, count := range topicCounts {
			dist[scope][topic] = float64(count) / float64(total)
		}
	}

	return dist
}

// BuildConfidenceHistograms builds 10-bin confidence histograms per domain classifier and globally.
func BuildConfidenceHistograms(docs []ClassifiedDoc) map[string][]float64 {
	bins := make(map[string][]int)
	totals := make(map[string]int)

	for _, doc := range docs {
		addToBin(bins, "global", doc.Confidence)
		totals["global"]++

		if doc.CrimeConfidence > 0 {
			addToBin(bins, "crime", doc.CrimeConfidence)
			totals["crime"]++
		}
		if doc.MiningConfidence > 0 {
			addToBin(bins, "mining", doc.MiningConfidence)
			totals["mining"]++
		}
	}

	histograms := make(map[string][]float64, len(bins))
	for scope, binCounts := range bins {
		total := totals[scope]
		hist := make([]float64, HistogramBins)
		if total > 0 {
			for i, count := range binCounts {
				hist[i] = float64(count) / float64(total)
			}
		}
		histograms[scope] = hist
	}

	return histograms
}

// BuildCrossMatrix builds a region/category proportion matrix with raw counts.
func BuildCrossMatrix(
	docs []ClassifiedDoc,
) (matrix map[string]map[string]float64, counts map[string]map[string]int) {
	rawCounts := make(map[string]map[string]int)
	regionTotals := make(map[string]int)

	for _, doc := range docs {
		if doc.SourceRegion == "" {
			continue
		}
		for _, topic := range doc.Topics {
			addCount(rawCounts, doc.SourceRegion, topic)
			regionTotals[doc.SourceRegion]++
		}
	}

	matrix = make(map[string]map[string]float64, len(rawCounts))
	for region, topicCounts := range rawCounts {
		matrix[region] = make(map[string]float64, len(topicCounts))
		total := regionTotals[region]
		if total == 0 {
			continue
		}
		for topic, count := range topicCounts {
			matrix[region][topic] = float64(count) / float64(total)
		}
	}

	return matrix, rawCounts
}

func addCount(m map[string]map[string]int, scope, key string) {
	if m[scope] == nil {
		m[scope] = make(map[string]int)
	}
	m[scope][key]++
}

func addToBin(bins map[string][]int, scope string, value float64) {
	if bins[scope] == nil {
		bins[scope] = make([]int, HistogramBins)
	}
	bin := int(value * float64(HistogramBins))
	if bin >= HistogramBins {
		bin = HistogramBins - 1
	}
	if bin < 0 {
		bin = 0
	}
	bins[scope][bin]++
}
