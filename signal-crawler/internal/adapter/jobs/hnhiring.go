package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	hnHiringTimeout      = 30 * time.Second
	hnHiringMaxBody      = 10 * 1024 * 1024 // 10 MB
	defaultAlgoliaURL    = "https://hn.algolia.com"
	defaultFirebaseURL   = "https://hacker-news.firebaseio.com"
	defaultHNMaxComments = 50
	hnCommentParts       = 2
	hnPipeParts          = 3
	hnMinParts           = 2
)

var htmlTagRegexp = regexp.MustCompile(`<[^>]*>`)

// HNHiringBoard fetches job postings from the monthly "Who is hiring?" HN thread.
type HNHiringBoard struct {
	algoliaURL  string
	firebaseURL string
	maxComments int
	httpClient  *http.Client
}

// NewHNHiring creates an HN Who's Hiring board parser.
func NewHNHiring(algoliaURL, firebaseURL string, maxComments int) *HNHiringBoard {
	if algoliaURL == "" {
		algoliaURL = defaultAlgoliaURL
	}
	if firebaseURL == "" {
		firebaseURL = defaultFirebaseURL
	}
	if maxComments <= 0 {
		maxComments = defaultHNMaxComments
	}
	return &HNHiringBoard{
		algoliaURL:  algoliaURL,
		firebaseURL: firebaseURL,
		maxComments: maxComments,
		httpClient:  &http.Client{Timeout: hnHiringTimeout},
	}
}

// Name returns the board identifier.
func (b *HNHiringBoard) Name() string { return "hn-hiring" }

// Fetch finds the latest "Who is hiring?" thread and extracts job postings from comments.
func (b *HNHiringBoard) Fetch(ctx context.Context) ([]Posting, error) {
	threadID, err := b.findLatestThread(ctx)
	if err != nil {
		return nil, err
	}

	kids, err := b.fetchThreadKids(ctx, threadID)
	if err != nil {
		return nil, err
	}

	if len(kids) > b.maxComments {
		kids = kids[:b.maxComments]
	}

	postings := make([]Posting, 0, len(kids))
	var fetchErrors int
	for _, kid := range kids {
		comment, fetchErr := b.fetchComment(ctx, kid)
		if fetchErr != nil {
			fetchErrors++
			continue
		}
		if p, ok := parseHNComment(comment, kid); ok {
			postings = append(postings, p)
		}
	}

	if fetchErrors > len(kids)/2 {
		return postings, fmt.Errorf("hn-hiring: %d/%d comment fetches failed", fetchErrors, len(kids))
	}

	return postings, nil
}

type algoliaSearchResult struct {
	Hits []struct {
		ObjectID string `json:"objectID"`
		Title    string `json:"title"`
	} `json:"hits"`
}

func (b *HNHiringBoard) findLatestThread(ctx context.Context) (int, error) {
	searchURL := b.algoliaURL + `/api/v1/search?query="Who+is+hiring"&tags=ask_hn&hitsPerPage=1`

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, http.NoBody)
	if err != nil {
		return 0, fmt.Errorf("hn-hiring: create search request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("hn-hiring: search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return 0, fmt.Errorf("hn-hiring: search HTTP %d", resp.StatusCode)
	}

	var result algoliaSearchResult
	if decErr := json.NewDecoder(io.LimitReader(resp.Body, hnHiringMaxBody)).Decode(&result); decErr != nil {
		return 0, fmt.Errorf("hn-hiring: decode search: %w", decErr)
	}

	if len(result.Hits) == 0 {
		return 0, errors.New("hn-hiring: no hiring thread found")
	}

	id, err := strconv.Atoi(result.Hits[0].ObjectID)
	if err != nil {
		return 0, fmt.Errorf("hn-hiring: parse thread ID: %w", err)
	}

	return id, nil
}

type hnItem struct {
	ID   int    `json:"id"`
	Kids []int  `json:"kids"`
	Text string `json:"text"`
}

func (b *HNHiringBoard) fetchThreadKids(ctx context.Context, threadID int) ([]int, error) {
	itemURL := fmt.Sprintf("%s/v0/item/%d.json", b.firebaseURL, threadID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, itemURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("hn-hiring: create thread request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hn-hiring: fetch thread: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("hn-hiring: thread HTTP %d", resp.StatusCode)
	}

	var item hnItem
	if decErr := json.NewDecoder(io.LimitReader(resp.Body, hnHiringMaxBody)).Decode(&item); decErr != nil {
		return nil, fmt.Errorf("hn-hiring: decode thread: %w", decErr)
	}

	return item.Kids, nil
}

func (b *HNHiringBoard) fetchComment(ctx context.Context, commentID int) (string, error) {
	itemURL := fmt.Sprintf("%s/v0/item/%d.json", b.firebaseURL, commentID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, itemURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("hn-hiring: create comment request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("hn-hiring: fetch comment %d: %w", commentID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("hn-hiring: comment HTTP %d", resp.StatusCode)
	}

	var item hnItem
	if decErr := json.NewDecoder(io.LimitReader(resp.Body, hnHiringMaxBody)).Decode(&item); decErr != nil {
		return "", fmt.Errorf("hn-hiring: decode comment %d: %w", commentID, decErr)
	}

	return item.Text, nil
}

// parseHNComment extracts company and title from "Company | Role | Location" format.
func parseHNComment(text string, commentID int) (Posting, bool) {
	if text == "" {
		return Posting{}, false
	}

	// Strip HTML tags
	clean := htmlTagRegexp.ReplaceAllString(text, "\n")

	// First line has the "Company | Role | Location" format
	lines := strings.SplitN(clean, "\n", hnCommentParts)
	firstLine := strings.TrimSpace(lines[0])

	parts := strings.SplitN(firstLine, "|", hnPipeParts)
	if len(parts) < hnMinParts {
		return Posting{}, false
	}

	company := strings.TrimSpace(parts[0])
	title := strings.TrimSpace(parts[1])

	body := ""
	if len(lines) > 1 {
		body = strings.TrimSpace(lines[1])
	}

	return Posting{
		Title:   title,
		Company: company,
		URL:     fmt.Sprintf("https://news.ycombinator.com/item?id=%d", commentID),
		ID:      strconv.Itoa(commentID),
		Body:    body,
		Sector:  "tech",
	}, true
}
