package enricher

// Registry maps requested enrichment types to implementations.
type Registry struct {
	byType map[string]Enricher
}

// NewRegistry constructs a registry from explicit enrichers.
func NewRegistry(enrichers ...Enricher) Registry {
	byType := make(map[string]Enricher, len(enrichers))
	for _, item := range enrichers {
		if item == nil {
			continue
		}
		byType[item.Type()] = item
	}
	return Registry{byType: byType}
}

// NewDefaultRegistry registers the only enrichment types supported by the mission.
func NewDefaultRegistry(searcher Searcher) Registry {
	return NewRegistry(
		NewCompanyIntel(searcher),
		NewTechStack(searcher),
		NewHiring(searcher),
	)
}

// Lookup returns an enricher and whether the requested type is known.
func (r Registry) Lookup(enrichmentType string) (Enricher, bool) {
	item, ok := r.byType[enrichmentType]
	return item, ok
}

// Types returns the registered enrichment type keys.
func (r Registry) Types() []string {
	types := make([]string, 0, len(r.byType))
	for key := range r.byType {
		types = append(types, key)
	}
	return types
}
