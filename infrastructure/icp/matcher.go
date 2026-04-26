package icp

import (
	"math"
	"slices"
	"strings"
)

type Document struct {
	Title      string
	Body       string
	SourceName string
	URL        string
	Topics     []string
}

type Result struct {
	Segments     []SegmentMatch `json:"segments"`
	ModelVersion string         `json:"model_version"`
}

type SegmentMatch struct {
	Segment         string   `json:"segment"`
	Score           float64  `json:"score"`
	MatchedKeywords []string `json:"matched_keywords"`
}

func Match(seed *Seed, doc Document) *Result {
	if seed == nil {
		return nil
	}
	text := strings.ToLower(strings.Join([]string{doc.Title, doc.Body, doc.SourceName, doc.URL}, " "))
	topics := normalizeTerms(doc.Topics)
	matches := make([]SegmentMatch, 0, len(seed.Segments))
	for _, segment := range seed.Segments {
		requiredAny := normalizeTerms(segment.RequiredAny)
		keywords := normalizeTerms(segment.Keywords)
		segmentTopics := normalizeTerms(segment.Topics)
		if len(requiredAny) > 0 && len(matchTerms(text, requiredAny)) == 0 {
			continue
		}
		keywordMatches := matchTerms(text, keywords)
		topicMatches := matchTopics(topics, segmentTopics)
		if len(keywordMatches) == 0 && len(topicMatches) == 0 {
			continue
		}
		matched := append(keywordMatches, topicMatches...)
		slices.Sort(matched)
		score := scoreSegment(len(keywordMatches), len(topicMatches), len(keywords), len(segmentTopics))
		if score < segment.MinScore {
			continue
		}
		matches = append(matches, SegmentMatch{
			Segment:         segment.Name,
			Score:           score,
			MatchedKeywords: matched,
		})
	}
	if len(matches) == 0 {
		return nil
	}
	slices.SortFunc(matches, func(a, b SegmentMatch) int {
		if a.Score == b.Score {
			return strings.Compare(a.Segment, b.Segment)
		}
		if a.Score > b.Score {
			return -1
		}
		return 1
	})
	return &Result{Segments: matches, ModelVersion: ModelVersionV1}
}

func matchTerms(text string, terms []string) []string {
	matches := make([]string, 0)
	for _, term := range terms {
		if strings.Contains(text, term) {
			matches = append(matches, term)
		}
	}
	return matches
}

func matchTopics(docTopics, segmentTopics []string) []string {
	matches := make([]string, 0)
	for _, topic := range segmentTopics {
		if slices.Contains(docTopics, topic) {
			matches = append(matches, "topic:"+topic)
		}
	}
	return matches
}

func scoreSegment(keywordMatches, topicMatches, keywordCount, topicCount int) float64 {
	keywordDenom := math.Max(float64(keywordCount), 1)
	topicDenom := math.Max(float64(topicCount), 1)
	keywordScore := math.Min(float64(keywordMatches)/keywordDenom*1.5, 0.85)
	topicScore := math.Min(float64(topicMatches)/topicDenom*0.5, 0.30)
	score := keywordScore + topicScore
	if keywordMatches >= 3 {
		score += 0.10
	}
	if score > 1 {
		score = 1
	}
	return math.Round(score*100) / 100
}
