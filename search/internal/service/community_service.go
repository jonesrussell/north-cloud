package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jonesrussell/north-cloud/search/internal/domain"
)

const (
	communitiesIndex         = "northcloud_communities"
	communitySearchMaxSize   = 10
	communitySearchMinLength = 1
)

// SearchCommunities queries the northcloud_communities index for autocomplete.
func (s *SearchService) SearchCommunities(
	ctx context.Context,
	query string,
	pageSize int,
) (*domain.CommunitySearchResponse, error) {
	query = strings.TrimSpace(query)
	if len(query) < communitySearchMinLength {
		return &domain.CommunitySearchResponse{Hits: make([]domain.CommunityHit, 0)}, nil
	}

	if pageSize <= 0 || pageSize > communitySearchMaxSize {
		pageSize = communitySearchMaxSize
	}

	esQuery := map[string]any{
		"size":    pageSize,
		"_source": []string{"id", "name", "community_type", "province"},
		"query": map[string]any{
			"match": map[string]any{
				"name": map[string]any{
					"query":    query,
					"analyzer": "autocomplete_search",
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(esQuery); err != nil {
		return nil, fmt.Errorf("encode community query: %w", err)
	}

	esClient := s.esClient.GetESClient()
	res, err := esClient.Search(
		esClient.Search.WithContext(ctx),
		esClient.Search.WithIndex(communitiesIndex),
		esClient.Search.WithBody(&buf),
		esClient.Search.WithTrackTotalHits(false),
	)
	if err != nil {
		return nil, fmt.Errorf("community search failed: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("community search error [%d]: %s", res.StatusCode, string(body))
	}

	return parseCommunityResponse(res.Body)
}

func parseCommunityResponse(body io.Reader) (*domain.CommunitySearchResponse, error) {
	var esResp struct {
		Hits struct {
			Hits []struct {
				Source struct {
					ID            string `json:"id"`
					Name          string `json:"name"`
					CommunityType string `json:"community_type"`
					Province      string `json:"province"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("decode community response: %w", err)
	}

	hits := make([]domain.CommunityHit, 0, len(esResp.Hits.Hits))
	for _, h := range esResp.Hits.Hits {
		hits = append(hits, domain.CommunityHit{
			ID:            h.Source.ID,
			Name:          h.Source.Name,
			CommunityType: h.Source.CommunityType,
			Province:      h.Source.Province,
		})
	}

	return &domain.CommunitySearchResponse{Hits: hits}, nil
}
