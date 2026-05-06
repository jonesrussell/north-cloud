package scope

import (
	"strings"

	"github.com/jonesrussell/indigenous-taxonomy/generated/go/taxonomy"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

// Resolver maps location hints to taxonomy slugs, walking the region hierarchy.
type Resolver struct {
	cityByName map[string]taxonomy.Region
}

// New returns a Resolver pre-loaded with the six supported city constants.
func New() *Resolver {
	return &Resolver{
		cityByName: map[string]taxonomy.Region{
			"winnipeg":  taxonomy.RegionCanadaManitobaWinnipeg,
			"toronto":   taxonomy.RegionCanadaOntarioToronto,
			"ottawa":    taxonomy.RegionCanadaOntarioOttawa,
			"vancouver": taxonomy.RegionCanadaBritishColumbiaVancouver,
			"calgary":   taxonomy.RegionCanadaAlbertaCalgary,
			"saskatoon": taxonomy.RegionCanadaSaskatchewanSaskatoon,
		},
	}
}

// maxAncestorDepth is the maximum number of ancestor slugs added per city
// (city + province + country = 3, plus one spare).
const maxAncestorDepth = 4

// Resolve returns a deduplicated slice of taxonomy slugs for the given source
// and optional location hint. The source's DefaultScope is always included
// first; if locationHint matches a known city the city slug and all ancestor
// slugs are appended (city → province → country).
func (r *Resolver) Resolve(source domain.AlertSource, locationHint string) []string {
	seen := make(map[string]struct{}, len(source.DefaultScope)+maxAncestorDepth)
	result := make([]string, 0, len(source.DefaultScope)+maxAncestorDepth)

	add := func(token string) {
		if token == "" {
			return
		}
		if _, dup := seen[token]; dup {
			return
		}
		seen[token] = struct{}{}
		result = append(result, token)
	}

	for _, t := range source.DefaultScope {
		add(t)
	}

	hint := strings.ToLower(strings.TrimSpace(locationHint))
	if hint == "" {
		return result
	}

	city, ok := r.cityByName[hint]
	if !ok {
		return result
	}

	add(string(city))

	current := city
	for {
		parent, parentOK := taxonomy.ParentRegion(current)
		if !parentOK {
			break
		}
		add(string(parent))
		current = parent
	}

	return result
}
