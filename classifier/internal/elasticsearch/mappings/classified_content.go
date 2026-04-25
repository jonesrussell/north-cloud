package mappings

import "github.com/jonesrussell/north-cloud/infrastructure/esmapping"

// ClassifiedContentMapping wraps the shared SSoT mapping for tooling and tests.
type ClassifiedContentMapping struct {
	doc map[string]any
}

// NewClassifiedContentMapping builds a classified_content index mapping with default shard/replica counts.
func NewClassifiedContentMapping() *ClassifiedContentMapping {
	return &ClassifiedContentMapping{doc: esmapping.ClassifiedContentIndex(1, 1)}
}

// GetJSON returns the classified content mapping as a JSON string.
func (m *ClassifiedContentMapping) GetJSON() (string, error) {
	return esmapping.ToIndentedJSON(m.doc)
}

// Validate validates the classified content mapping configuration.
func (m *ClassifiedContentMapping) Validate() error {
	return ValidateSettings(DefaultSettings())
}
