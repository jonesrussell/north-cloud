package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
)

// minSimilarityScore is the minimum name similarity ratio (0–1) required for a match.
const minSimilarityScore = 0.85

// indigenousKeywords are terms that indicate a source is indigenous-related.
var indigenousKeywords = []string{
	"first nation", "first nations", "band", "indigenous", "aboriginal",
	"tribal", "reserve", "treaty", "métis", "metis", "inuit",
	"anishinaabe", "anishinabe", "ojibwe", "cree", "mohawk",
}

// nameStripWords are words stripped when normalizing community/source names for matching.
var nameStripWords = []string{
	"first nation", "first nations", "band", "council",
	"indian band", "of", "the", "nation",
}

// SourceMatch represents a matched source-community pair.
type SourceMatch struct {
	CommunityID   string  `json:"community_id"`
	CommunityName string  `json:"community_name"`
	SourceID      string  `json:"source_id"`
	SourceName    string  `json:"source_name"`
	SourceURL     string  `json:"source_url"`
	Similarity    float64 `json:"similarity"`
}

// LinkerHandler handles source-to-community linking.
type LinkerHandler struct {
	communityRepo *repository.CommunityRepository
	sourceRepo    *repository.SourceRepository
	logger        infralogger.Logger
}

// NewLinkerHandler creates a new LinkerHandler.
func NewLinkerHandler(
	communityRepo *repository.CommunityRepository,
	sourceRepo *repository.SourceRepository,
	log infralogger.Logger,
) *LinkerHandler {
	return &LinkerHandler{
		communityRepo: communityRepo,
		sourceRepo:    sourceRepo,
		logger:        log,
	}
}

// LinkSources matches sources to communities and optionally applies the links.
func (h *LinkerHandler) LinkSources(c *gin.Context) {
	dryRun := c.DefaultQuery("dry_run", trueString) == trueString

	ctx := c.Request.Context()

	matches, err := h.findMatches(ctx)
	if err != nil {
		h.logger.Error("Failed to find source-community matches", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to match sources"})
		return
	}

	if dryRun {
		c.JSON(http.StatusOK, gin.H{
			"dry_run": true,
			"matches": matches,
			"count":   len(matches),
		})
		return
	}

	linked := h.applyMatches(ctx, matches)

	c.JSON(http.StatusOK, gin.H{
		"dry_run": false,
		"matches": matches,
		"count":   len(matches),
		"linked":  linked,
	})
}

// findMatches loads sources and unlinked communities, then runs the matching algorithm.
func (h *LinkerHandler) findMatches(ctx context.Context) ([]SourceMatch, error) {
	sources, err := h.sourceRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	communities, communityErr := h.communityRepo.ListUnlinked(ctx)
	if communityErr != nil {
		return nil, communityErr
	}

	if len(communities) == 0 || len(sources) == 0 {
		return []SourceMatch{}, nil
	}

	indigenousSources := filterIndigenousSources(sources)
	return matchSourcesToCommunities(indigenousSources, communities), nil
}

// applyMatches persists each match by setting source_id and website on the community.
func (h *LinkerHandler) applyMatches(ctx context.Context, matches []SourceMatch) int {
	linked := 0
	for i := range matches {
		if err := h.communityRepo.SetSourceLink(
			ctx, matches[i].CommunityID, matches[i].SourceID, matches[i].SourceURL,
		); err != nil {
			h.logger.Error("Failed to link source",
				infralogger.String("community_id", matches[i].CommunityID),
				infralogger.String("source_id", matches[i].SourceID),
				infralogger.Error(err),
			)
			continue
		}
		linked++
	}
	return linked
}

// filterIndigenousSources returns sources whose name or URL contains indigenous keywords.
func filterIndigenousSources(sources []models.Source) []models.Source {
	filtered := make([]models.Source, 0, len(sources))
	for i := range sources {
		nameLower := strings.ToLower(sources[i].Name)
		urlLower := strings.ToLower(sources[i].URL)
		for _, kw := range indigenousKeywords {
			if strings.Contains(nameLower, kw) || strings.Contains(urlLower, kw) {
				filtered = append(filtered, sources[i])
				break
			}
		}
	}
	return filtered
}

// matchSourcesToCommunities runs the two-gate matching algorithm.
func matchSourcesToCommunities(sources []models.Source, communities []models.Community) []SourceMatch {
	matches := make([]SourceMatch, 0)
	for i := range communities {
		normCommunity := normalizeName(communities[i].Name)
		if normCommunity == "" {
			continue
		}
		bestMatch := findBestSourceMatch(normCommunity, sources)
		if bestMatch == nil {
			continue
		}
		matches = append(matches, SourceMatch{
			CommunityID:   communities[i].ID,
			CommunityName: communities[i].Name,
			SourceID:      bestMatch.source.ID,
			SourceName:    bestMatch.source.Name,
			SourceURL:     bestMatch.source.URL,
			Similarity:    bestMatch.score,
		})
	}
	return matches
}

type sourceMatchCandidate struct {
	source models.Source
	score  float64
}

// findBestSourceMatch finds the best matching source for a normalized community name.
func findBestSourceMatch(normCommunity string, sources []models.Source) *sourceMatchCandidate {
	communityWords := strings.Fields(normCommunity)
	if len(communityWords) == 0 {
		return nil
	}
	firstWord := communityWords[0]

	var best *sourceMatchCandidate
	for i := range sources {
		normSource := normalizeName(sources[i].Name)
		if normSource == "" {
			continue
		}
		// Gate 1: first significant word must appear in source name
		if !strings.Contains(normSource, firstWord) {
			continue
		}
		// Gate 2: similarity score
		score := similarText(normCommunity, normSource)
		if score < minSimilarityScore {
			continue
		}
		if best == nil || score > best.score {
			best = &sourceMatchCandidate{source: sources[i], score: score}
		}
	}
	return best
}

// normalizeName strips common suffixes and lowercases for comparison.
func normalizeName(name string) string {
	result := strings.ToLower(strings.TrimSpace(name))
	for _, word := range nameStripWords {
		result = strings.ReplaceAll(result, word, "")
	}
	// Collapse whitespace
	fields := strings.Fields(result)
	return strings.Join(fields, " ")
}

// similarText computes a similarity ratio between two strings (0–1),
// using longest common substring recursion (similar to PHP's similar_text).
func similarText(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	total := float64(len(a) + len(b))
	matching := float64(similarTextCount(a, b))
	return (matching * 2) / total //nolint:mnd // standard similarity formula: 2*matches/total
}

// similarTextCount returns the number of matching characters using longest common substring.
func similarTextCount(a, b string) int {
	if a == "" || b == "" {
		return 0
	}
	longest := 0
	longestA, longestB := 0, 0

	for i := range a {
		for j := range b {
			l := 0
			for i+l < len(a) && j+l < len(b) && a[i+l] == b[j+l] {
				l++
			}
			if l > longest {
				longest = l
				longestA = i
				longestB = j
			}
		}
	}

	if longest == 0 {
		return 0
	}

	count := longest
	count += similarTextCount(a[:longestA], b[:longestB])
	count += similarTextCount(a[longestA+longest:], b[longestB+longest:])
	return count
}
