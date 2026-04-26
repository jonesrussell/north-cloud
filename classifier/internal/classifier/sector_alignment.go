package classifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/infrastructure/icp"
)

const defaultSectorAlignmentRefreshInterval = 30 * time.Second

type SectorAlignmentExtractor struct {
	provider seedProvider
}

type seedProvider interface {
	Seed(context.Context) (*icp.Seed, error)
}

func NewSectorAlignmentExtractor(provider seedProvider) *SectorAlignmentExtractor {
	return &SectorAlignmentExtractor{provider: provider}
}

func NewHTTPICPSeedProvider(baseURL string, refreshInterval time.Duration, client *http.Client) *HTTPICPSeedProvider {
	if refreshInterval == 0 {
		refreshInterval = defaultSectorAlignmentRefreshInterval
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &HTTPICPSeedProvider{
		url:             strings.TrimRight(baseURL, "/") + "/api/v1/icp-segments",
		refreshInterval: refreshInterval,
		client:          client,
	}
}

type HTTPICPSeedProvider struct {
	url             string
	refreshInterval time.Duration
	client          *http.Client

	mu        sync.Mutex
	cached    *icp.Seed
	fetchedAt time.Time
}

func (p *HTTPICPSeedProvider) Seed(ctx context.Context) (*icp.Seed, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cached != nil && time.Since(p.fetchedAt) < p.refreshInterval {
		return p.cached, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return nil, err
	}
	res, err := p.client.Do(req)
	if err != nil {
		if p.cached != nil {
			return p.cached, nil
		}
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		if p.cached != nil {
			return p.cached, nil
		}
		return nil, fmt.Errorf("source-manager ICP seed returned status %d", res.StatusCode)
	}
	var seed icp.Seed
	if err = json.NewDecoder(res.Body).Decode(&seed); err != nil {
		return nil, err
	}
	if err = icp.ValidateSeed(&seed); err != nil {
		return nil, err
	}
	p.cached = &seed
	p.fetchedAt = time.Now()
	return p.cached, nil
}

type StaticICPSeedProvider struct {
	SeedValue *icp.Seed
}

func (p StaticICPSeedProvider) Seed(context.Context) (*icp.Seed, error) {
	return p.SeedValue, nil
}

func (e *SectorAlignmentExtractor) Extract(
	ctx context.Context, raw *domain.RawContent, topics []string,
) (*domain.ICPResult, error) {
	seed, err := e.provider.Seed(ctx)
	if err != nil {
		return nil, err
	}
	result := icp.Match(seed, icp.Document{
		Title:      raw.Title,
		Body:       raw.RawText,
		SourceName: raw.SourceName,
		URL:        raw.URL,
		Topics:     topics,
	})
	if result == nil {
		return nil, nil
	}
	segments := make([]domain.ICPSegmentResult, 0, len(result.Segments))
	for _, segment := range result.Segments {
		segments = append(segments, domain.ICPSegmentResult{
			Segment:         segment.Segment,
			Score:           segment.Score,
			MatchedKeywords: segment.MatchedKeywords,
		})
	}
	return &domain.ICPResult{
		Segments:     segments,
		ModelVersion: result.ModelVersion,
	}, nil
}
