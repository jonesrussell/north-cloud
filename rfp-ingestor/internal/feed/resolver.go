package feed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	resolverHTTPTimeout = 15 * time.Second

	// seaoCKANDatasetID is the Données Québec CKAN dataset identifier for SEAO.
	seaoCKANDatasetID = "systeme-electronique-dappel-doffres-seao"
)

// URLResolver dynamically resolves feed URLs before each poll cycle.
type URLResolver interface {
	// Resolve returns the current download URL(s) for a feed source.
	Resolve(ctx context.Context) ([]string, error)
}

// NewResolver creates a URLResolver for the given resolver name.
// Returns nil if the name is empty or unrecognized.
func NewResolver(name, baseURL string) URLResolver {
	switch name {
	case "seao_ckan":
		apiURL := baseURL
		if apiURL == "" {
			apiURL = "https://www.donneesquebec.ca"
		}
		return &seaoCKANResolver{
			apiURL:     apiURL,
			httpClient: &http.Client{Timeout: resolverHTTPTimeout},
		}
	default:
		return nil
	}
}

// seaoCKANResolver queries the Données Québec CKAN API to discover the latest
// weekly SEAO JSON resource.
type seaoCKANResolver struct {
	apiURL     string
	httpClient *http.Client
}

func (r *seaoCKANResolver) Resolve(ctx context.Context) ([]string, error) {
	url := fmt.Sprintf("%s/recherche/api/3/action/package_show?id=%s", r.apiURL, seaoCKANDatasetID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("seao resolver: create request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("seao resolver: fetch CKAN API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("seao resolver: CKAN API returned HTTP %d", resp.StatusCode)
	}

	var result ckanPackageResult
	if decErr := json.NewDecoder(resp.Body).Decode(&result); decErr != nil {
		return nil, fmt.Errorf("seao resolver: decode CKAN response: %w", decErr)
	}

	if !result.Success {
		return nil, errors.New("seao resolver: CKAN API returned success=false")
	}

	downloadURL, err := findLatestHebdoURL(result.Result.Resources)
	if err != nil {
		return nil, fmt.Errorf("seao resolver: %w", err)
	}

	return []string{downloadURL}, nil
}

// findLatestHebdoURL finds the most recent hebdo_*.json resource by name.
func findLatestHebdoURL(resources []ckanResource) (string, error) {
	var hebdos []ckanResource

	for _, res := range resources {
		name := strings.ToLower(res.Name)
		url := strings.ToLower(res.URL)

		if (strings.Contains(name, "hebdo") || strings.Contains(url, "hebdo")) &&
			strings.HasSuffix(url, ".json") {
			hebdos = append(hebdos, res)
		}
	}

	if len(hebdos) == 0 {
		return "", errors.New("no hebdo JSON resources found in CKAN dataset")
	}

	// Sort by name descending — hebdo filenames contain dates (hebdo_YYYYMMDD_YYYYMMDD).
	sort.Slice(hebdos, func(i, j int) bool {
		return hebdos[i].Name > hebdos[j].Name
	})

	return hebdos[0].URL, nil
}

// CKAN API response types.

type ckanPackageResult struct {
	Success bool        `json:"success"`
	Result  ckanPackage `json:"result"`
}

type ckanPackage struct {
	Resources []ckanResource `json:"resources"`
}

type ckanResource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}
