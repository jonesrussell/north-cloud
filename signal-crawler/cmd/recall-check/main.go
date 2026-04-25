package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter/jobs"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/config"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/scoring"
)

const (
	defaultHTTPTimeout = 30 * time.Second
	defaultRunTimeout  = 5 * time.Minute
	maxBodyBytes       = 10 * 1024 * 1024
	thresholdOne       = 1
	thresholdTwo       = 2
	percentMultiplier  = 100
	reviewDropPercent  = 40
)

var htmlTagRegexp = regexp.MustCompile(`<[^>]*>`)

type result struct {
	Source             string   `json:"source"`
	Scanned            int      `json:"scanned"`
	AcceptedThreshold1 int      `json:"accepted_threshold_1"`
	AcceptedThreshold2 int      `json:"accepted_threshold_2"`
	DropPercent        float64  `json:"drop_percent"`
	SingleHitPhrases   []phrase `json:"single_hit_phrases,omitempty"`
	Errors             []string `json:"errors,omitempty"`
}

type phrase struct {
	Phrase string `json:"phrase"`
	Count  int    `json:"count"`
}

type hnItem struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

func main() {
	configPath := flag.String("config", "config.yml", "Path to signal-crawler config.yml")
	jsonOut := flag.Bool("json", false, "Print machine-readable JSON")
	timeout := flag.Duration("timeout", defaultRunTimeout, "Overall fetch timeout")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "recall-check: load config: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := &http.Client{Timeout: defaultHTTPTimeout}
	jobResults := scanJobBoards(ctx, cfg)
	results := make([]result, 0, thresholdOne+len(jobResults))
	results = append(results, scanHN(ctx, client, cfg))
	results = append(results, jobResults...)
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Source < results[j].Source
	})

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if encodeErr := enc.Encode(results); encodeErr != nil {
			fmt.Fprintf(os.Stderr, "recall-check: encode json: %v\n", encodeErr)
			os.Exit(1)
		}
		return
	}

	printTable(results)
	printRecommendation(results)
}

func scanHN(ctx context.Context, client *http.Client, cfg *config.Config) result {
	res := result{Source: "hn"}
	ids, err := fetchHNIDs(ctx, client, cfg.HN.BaseURL)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		return res
	}
	if cfg.HN.MaxItems > 0 && len(ids) > cfg.HN.MaxItems {
		ids = ids[:cfg.HN.MaxItems]
	}
	res.Scanned = len(ids)

	for _, id := range ids {
		item, itemErr := fetchHNItem(ctx, client, cfg.HN.BaseURL, id)
		if itemErr != nil {
			res.Errors = append(res.Errors, itemErr.Error())
			continue
		}
		countThresholds(cleanText(item.Title+" "+item.Text), &res)
	}
	finish(&res)
	return res
}

func fetchHNIDs(ctx context.Context, client *http.Client, baseURL string) ([]int, error) {
	storiesURL := strings.TrimRight(baseURL, "/") + "/v0/newstories.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, storiesURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("hn: create newstories request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hn: fetch newstories: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("hn: newstories HTTP %d", resp.StatusCode)
	}

	var ids []int
	if decodeErr := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(&ids); decodeErr != nil {
		return nil, fmt.Errorf("hn: decode newstories: %w", decodeErr)
	}
	return ids, nil
}

func fetchHNItem(ctx context.Context, client *http.Client, baseURL string, id int) (hnItem, error) {
	itemURL := strings.TrimRight(baseURL, "/") + "/v0/item/" + strconv.Itoa(id) + ".json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, itemURL, http.NoBody)
	if err != nil {
		return hnItem{}, fmt.Errorf("hn: create item request %d: %w", id, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return hnItem{}, fmt.Errorf("hn: fetch item %d: %w", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		return hnItem{}, fmt.Errorf("hn: item %d HTTP %d", id, resp.StatusCode)
	}

	var item hnItem
	if decodeErr := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(&item); decodeErr != nil {
		return hnItem{}, fmt.Errorf("hn: decode item %d: %w", id, decodeErr)
	}
	return item, nil
}

func scanJobBoards(ctx context.Context, cfg *config.Config) []result {
	boards := []jobs.Board{
		jobs.NewRemoteOK(cfg.Jobs.RemoteOKURL),
		jobs.NewWWR(cfg.Jobs.WWRURL),
		jobs.NewHNHiring("", "", cfg.Jobs.HNMaxComments),
	}
	if !cfg.Jobs.GCJobsDisabled {
		boards = append(boards, jobs.NewGCJobs(cfg.Jobs.GCJobsURL, nil))
	}
	boards = append(boards, jobs.NewWorkBC(cfg.Jobs.WorkBCURL, nil))

	results := make([]result, 0, len(boards))
	for _, board := range boards {
		res := result{Source: "jobs/" + board.Name()}
		postings, err := board.Fetch(ctx)
		if err != nil {
			res.Errors = append(res.Errors, err.Error())
		}
		res.Scanned = len(postings)
		for _, posting := range postings {
			countThresholds(cleanText(posting.Title+" "+posting.Body), &res)
		}
		finish(&res)
		results = append(results, res)
	}
	return results
}

func countThresholds(text string, res *result) {
	phrases := scoring.MatchedPhrases(text)
	if len(phrases) >= thresholdOne {
		res.AcceptedThreshold1++
	}
	if len(phrases) >= thresholdTwo {
		res.AcceptedThreshold2++
	}
	if len(phrases) == thresholdOne {
		addPhrase(res, phrases[0])
	}
}

func cleanText(text string) string {
	return htmlTagRegexp.ReplaceAllString(text, " ")
}

func finish(res *result) {
	sort.SliceStable(res.SingleHitPhrases, func(i, j int) bool {
		if res.SingleHitPhrases[i].Count == res.SingleHitPhrases[j].Count {
			return res.SingleHitPhrases[i].Phrase < res.SingleHitPhrases[j].Phrase
		}
		return res.SingleHitPhrases[i].Count > res.SingleHitPhrases[j].Count
	})
	if res.AcceptedThreshold1 == 0 {
		return
	}
	dropped := res.AcceptedThreshold1 - res.AcceptedThreshold2
	res.DropPercent = float64(dropped) / float64(res.AcceptedThreshold1) * percentMultiplier
}

func addPhrase(res *result, matched string) {
	for i := range res.SingleHitPhrases {
		if res.SingleHitPhrases[i].Phrase == matched {
			res.SingleHitPhrases[i].Count++
			return
		}
	}
	res.SingleHitPhrases = append(res.SingleHitPhrases, phrase{Phrase: matched, Count: 1})
}

func printTable(results []result) {
	fmt.Println("| source | scanned | accepted @1 | accepted @2 | drop | errors |")
	fmt.Println("|---|---:|---:|---:|---:|---:|")
	for _, r := range results {
		fmt.Printf("| %s | %d | %d | %d | %.1f%% | %d |\n",
			r.Source,
			r.Scanned,
			r.AcceptedThreshold1,
			r.AcceptedThreshold2,
			r.DropPercent,
			len(r.Errors),
		)
	}
}

func printRecommendation(results []result) {
	var highDrop []string
	for _, r := range results {
		if r.AcceptedThreshold1 > 0 && r.DropPercent > reviewDropPercent {
			detail := ""
			if len(r.SingleHitPhrases) > 0 {
				detail = "; top one-hit phrase: " + r.SingleHitPhrases[0].Phrase
			}
			highDrop = append(highDrop, fmt.Sprintf("%s (%.1f%%%s)", r.Source, r.DropPercent, detail))
		}
	}

	if len(highDrop) == 0 {
		fmt.Println()
		fmt.Println("Recommendation: accept the unified 2-keyword threshold for this sample; no adapter exceeded the 40% drop review line.")
		return
	}

	fmt.Println()
	fmt.Printf(
		"Recommendation: review %s before accepting the 2-keyword gate unchanged. "+
			"Options: broaden the keyword list for missed canonical phrases, relax signal-crawler per adapter, "+
			"or document the accepted drop in docs/specs/lead-pipeline.md.\n",
		strings.Join(highDrop, ", "),
	)
}
