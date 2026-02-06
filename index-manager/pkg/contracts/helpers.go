package contracts

// extractProperties navigates the standard ES mapping structure to extract the
// properties map: { "mappings": { "properties": { ... } } }
func extractProperties(full map[string]any) Mapping {
	mappingsObj, ok := full["mappings"].(map[string]any)
	if !ok {
		return Mapping{Properties: map[string]any{}}
	}

	props, ok := mappingsObj["properties"].(map[string]any)
	if !ok {
		return Mapping{Properties: map[string]any{}}
	}

	return Mapping{Properties: props}
}
