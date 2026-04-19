package funding

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	infrasignal "github.com/jonesrussell/north-cloud/infrastructure/signal"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/scoring"
)

const (
	defaultHTTPTimeout = 30 * time.Second
	maxResponseBytes   = 10 * 1024 * 1024 // 10 MB
	// Only emit signals for grants approved in the last 90 days.
	recentGrantWindow = 90 * 24 * time.Hour
)

// CSV column indices (0-based) for the OTF open-data CSV.
const (
	colGrantProgramme = 4
	colIdentifier     = 7
	colOrgName        = 8
	colApprovalDate   = 10
	colAmountAwarded  = 12
	colDescription    = 14
	colCity           = 20
	colGrantStatus    = 29
)

// Adapter fetches government grant data from OTF's open-data CSV.
type Adapter struct {
	urls       []string
	httpClient *http.Client
}

// New creates a new funding Adapter that will fetch the given CSV URLs.
func New(urls []string) *Adapter {
	return &Adapter{
		urls:       urls,
		httpClient: &http.Client{Timeout: defaultHTTPTimeout},
	}
}

// Name returns the short identifier for this adapter.
func (a *Adapter) Name() string {
	return "funding"
}

// Scan fetches each configured URL, parses grant CSV rows, and returns signals.
// Continues on per-URL errors, returning partial results with a combined error.
func (a *Adapter) Scan(ctx context.Context) ([]adapter.Signal, error) {
	var allSignals []adapter.Signal
	var errs []error

	for _, rawURL := range a.urls {
		grants, err := a.fetchAndParse(ctx, rawURL)
		if err != nil {
			errs = append(errs, fmt.Errorf("funding adapter: fetch %s: %w", rawURL, err))
			continue
		}
		allSignals = append(allSignals, grants...)
	}

	return allSignals, errors.Join(errs...)
}

func (a *Adapter) fetchAndParse(ctx context.Context, rawURL string) ([]adapter.Signal, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("funding: create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("funding: fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("funding adapter: HTTP %d fetching %s", resp.StatusCode, rawURL)
	}

	return parseCSV(io.LimitReader(resp.Body, maxResponseBytes), rawURL)
}

// parseCSV reads OTF open-data CSV and returns signals for recent active grants.
// sourceURL is the feed URL the CSV was fetched from, used as the URL fallback
// for organization attribution.
func parseCSV(r io.Reader, sourceURL string) ([]adapter.Signal, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1 // OTF CSV has inconsistent field counts in some rows

	// Skip header row.
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("funding: read CSV header: %w", err)
	}
	if len(header) < colGrantStatus+1 {
		return nil, fmt.Errorf("funding: CSV has %d columns, expected at least %d", len(header), colGrantStatus+1)
	}

	cutoff := time.Now().Add(-recentGrantWindow)
	var signals []adapter.Signal

	for {
		record, readErr := reader.Read()
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return signals, fmt.Errorf("funding: read CSV row: %w", readErr)
		}

		if len(record) < colGrantStatus+1 {
			continue
		}

		// Only process active grants.
		if !strings.EqualFold(strings.TrimSpace(record[colGrantStatus]), "Active") {
			continue
		}

		// Only process recently approved grants.
		approved, parseErr := time.Parse("2006-01-02", strings.TrimSpace(record[colApprovalDate]))
		if parseErr != nil || approved.Before(cutoff) {
			continue
		}

		org := strings.TrimSpace(record[colOrgName])
		programme := strings.TrimSpace(record[colGrantProgramme])
		amount := strings.TrimSpace(record[colAmountAwarded])
		identifier := strings.TrimSpace(record[colIdentifier])
		city := strings.TrimSpace(record[colCity])

		if org == "" || programme == "" {
			continue
		}

		label := fmt.Sprintf("%s — %s", org, programme)
		notes := fmt.Sprintf("Received $%s in %s. Likely needs tech implementation.", amount, city)

		// Organization is explicit in the CSV; fall back to the feed URL only if Normalize
		// rejects it (rare — corporate-suffix-only strings).
		orgNormalized, _ := infrasignal.Resolve(org, "", sourceURL)

		signals = append(signals, adapter.Signal{
			SignalType:        "funding_win",
			SourceName:        "funding",
			Label:             label,
			ExternalID:        url.QueryEscape(identifier),
			SourceURL:         "https://otf.ca/our-grants/grants-awarded",
			SignalStrength:    scoring.ScoreStrongSignal,
			FundingStatus:     "awarded",
			Notes:             notes,
			OrgName:           org,
			OrgNameNormalized: orgNormalized,
		})
	}

	return signals, nil
}
