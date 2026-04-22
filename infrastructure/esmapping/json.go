package esmapping

import (
	"encoding/json"
	"fmt"
)

// ToIndentedJSON marshals a mapping document for stable snapshots and tooling.
func ToIndentedJSON(v any) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal mapping: %w", err)
	}
	return string(data), nil
}
