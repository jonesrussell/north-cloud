package icp

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

const ModelVersionV1 = "v1"

const expectedSegmentCount = 3

type Seed struct {
	SegmentSchemaVersion int       `json:"segment_schema_version" yaml:"segment_schema_version"`
	SeedUpdatedAt        string    `json:"seed_updated_at"        yaml:"seed_updated_at"`
	Segments             []Segment `json:"segments"               yaml:"segments"`
}

type Segment struct {
	Name        string   `json:"name"                   yaml:"name"`
	Description string   `json:"description"            yaml:"description"`
	Keywords    []string `json:"keywords"               yaml:"keywords"`
	Topics      []string `json:"topics"                 yaml:"topics"`
	RequiredAny []string `json:"required_any,omitempty" yaml:"required_any,omitempty"`
	MinScore    float64  `json:"min_score"              yaml:"min_score"`
}

func LoadSeed(path string) (*Seed, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ICP seed: %w", err)
	}
	var seed Seed
	if err = yaml.Unmarshal(data, &seed); err != nil {
		return nil, fmt.Errorf("parse ICP seed: %w", err)
	}
	if validateErr := ValidateSeed(&seed); validateErr != nil {
		return nil, validateErr
	}
	return &seed, nil
}

func ValidateSeed(seed *Seed) error {
	if seed == nil {
		return errors.New("ICP seed is nil")
	}
	if seed.SegmentSchemaVersion != 1 {
		return fmt.Errorf("segment_schema_version must be 1, got %d", seed.SegmentSchemaVersion)
	}
	if strings.TrimSpace(seed.SeedUpdatedAt) == "" {
		return errors.New("seed_updated_at is required")
	}
	if len(seed.Segments) != expectedSegmentCount {
		return fmt.Errorf("expected exactly %d ICP segments, got %d", expectedSegmentCount, len(seed.Segments))
	}

	return validateSegments(seed.Segments)
}

func validateSegments(segments []Segment) error {
	allowed := map[string]bool{
		"indigenous_channel":        true,
		"northern_ontario_industry": true,
		"private_sector_smb":        true,
	}
	seen := make(map[string]bool, len(segments))
	for _, segment := range segments {
		name := strings.TrimSpace(segment.Name)
		if err := validateSegment(name, segment, allowed, seen); err != nil {
			return err
		}
		seen[name] = true
	}
	for name := range allowed {
		if !seen[name] {
			return fmt.Errorf("missing ICP segment %q", name)
		}
	}
	return nil
}

func validateSegment(
	name string,
	segment Segment,
	allowed map[string]bool,
	seen map[string]bool,
) error {
	switch {
	case !allowed[name]:
		return fmt.Errorf("unknown ICP segment %q", segment.Name)
	case seen[name]:
		return fmt.Errorf("duplicate ICP segment %q", name)
	case strings.TrimSpace(segment.Description) == "":
		return fmt.Errorf("segment %q description is required", name)
	case len(segment.Keywords) == 0 && len(segment.Topics) == 0:
		return fmt.Errorf("segment %q needs at least one keyword or topic", name)
	case segment.MinScore <= 0 || segment.MinScore > maxSegmentScore:
		return fmt.Errorf("segment %q min_score must be > 0 and <= 1", name)
	case hasBlank(segment.Keywords) || hasBlank(segment.Topics) || hasBlank(segment.RequiredAny):
		return fmt.Errorf("segment %q contains a blank keyword/topic/required term", name)
	default:
		return nil
	}
}

func normalizeTerms(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		term := strings.ToLower(strings.TrimSpace(value))
		if term == "" || seen[term] {
			continue
		}
		seen[term] = true
		out = append(out, term)
	}
	slices.Sort(out)
	return out
}

func hasBlank(values []string) bool {
	return slices.ContainsFunc(values, func(value string) bool {
		return strings.TrimSpace(value) == ""
	})
}
