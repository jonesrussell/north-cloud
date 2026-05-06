package domain

import (
	"encoding/json"
	"errors"
	"time"
)

// HazardType classifies the substance-supply hazard.
type HazardType string

const (
	HazardOpioidSupply    HazardType = "opioid_supply"
	HazardStimulantSupply HazardType = "stimulant_supply"
	HazardBenzoSupply     HazardType = "benzo_supply"
	HazardOther           HazardType = "other"
)

// Hazard is the discriminated union carrier for category-specific hazard data.
// v1 only implements HarmReductionHazard. MarshalJSON/UnmarshalJSON flatten the
// inner struct directly into the JSON hazard object (no nesting key).
type Hazard struct {
	HarmReduction *HarmReductionHazard
}

// HarmReductionHazard carries harm-reduction-specific fields.
// Matches the HarmReductionHazard definition in community-alert.schema.json.
type HarmReductionHazard struct {
	HazardType        HazardType  `json:"hazard_type"`
	Substances        []string    `json:"substances"`
	Composition       []Substance `json:"composition,omitempty"`
	VisualDescription string      `json:"visual_description,omitempty"`
	LabSource         string      `json:"lab_source,omitempty"`
	ConfirmationDate  *time.Time  `json:"confirmation_date,omitempty"`
}

// Substance is a single chemical constituent of a drug supply sample.
type Substance struct {
	Name               string  `json:"name"`
	Percentage         float64 `json:"percentage,omitempty"`
	IsActiveIngredient bool    `json:"is_active_ingredient,omitempty"`
	Note               string  `json:"note,omitempty"`
}

// MarshalJSON flattens the active inner hazard directly into the JSON object.
// The wire format carries no wrapping key — it matches the schema's oneOf shape.
func (h Hazard) MarshalJSON() ([]byte, error) {
	if h.HarmReduction == nil {
		return nil, errors.New("hazard: HarmReduction must not be nil for v1 alerts")
	}

	return json.Marshal(h.HarmReduction)
}

// UnmarshalJSON reads the flattened JSON hazard object into HarmReductionHazard.
// For v1, all hazards are harm_reduction; future categories will add a switch on
// a discriminator field read from the raw bytes before full decode.
func (h *Hazard) UnmarshalJSON(data []byte) error {
	var hr HarmReductionHazard
	if unmarshalErr := json.Unmarshal(data, &hr); unmarshalErr != nil {
		return unmarshalErr
	}

	h.HarmReduction = &hr

	return nil
}

// Validate returns an error when the Hazard carries no inner payload.
func (h *Hazard) Validate() error {
	if h.HarmReduction == nil {
		return errors.New("hazard: at least one hazard payload is required (v1: HarmReduction)")
	}

	return nil
}
