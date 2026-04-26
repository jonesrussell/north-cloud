package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jonesrussell/north-cloud/infrastructure/icp"
	"gopkg.in/yaml.v3"
)

type labelsFile struct {
	SegmentSchemaVersion int      `yaml:"segment_schema_version"`
	Segments             []string `yaml:"segments"`
	Labels               []label  `yaml:"labels"`
}

type label struct {
	DocID              string            `yaml:"doc_id"`
	ESIndex            string            `yaml:"es_index"`
	IngestTimestamp    string            `yaml:"ingest_timestamp"`
	Title              string            `yaml:"title"`
	Excerpt            string            `yaml:"excerpt"`
	Segments           map[string]string `yaml:"segments"`
	LabellerConfidence string            `yaml:"labeller_confidence"`
	Labeller           string            `yaml:"labeller"`
	LabelledAt         string            `yaml:"labelled_at"`
}

func main() {
	seedPath := flag.String("seed", "source-manager/data/icp-segments.yml", "path to ICP seed YAML")
	labelsPath := flag.String("labels", "classifier/testdata/icp_labels.yml", "path to ICP labels YAML")
	flag.Parse()

	if _, err := icp.LoadSeed(*seedPath); err != nil {
		fail("seed validation failed: %v", err)
	}
	if err := validateLabels(*labelsPath); err != nil {
		fail("labels validation failed: %v", err)
	}
	fmt.Println("ICP seed and labels validation passed")
}

func validateLabels(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var labels labelsFile
	if err = yaml.Unmarshal(data, &labels); err != nil {
		return err
	}
	if labels.SegmentSchemaVersion != 1 {
		return fmt.Errorf("segment_schema_version must be 1, got %d", labels.SegmentSchemaVersion)
	}
	requiredSegments := []string{"indigenous_channel", "northern_ontario_industry", "private_sector_smb"}
	if len(labels.Segments) != len(requiredSegments) {
		return fmt.Errorf("expected %d segments, got %d", len(requiredSegments), len(labels.Segments))
	}
	for _, segment := range requiredSegments {
		if !contains(labels.Segments, segment) {
			return fmt.Errorf("missing segment %q", segment)
		}
	}
	if len(labels.Labels) == 0 {
		return fmt.Errorf("labels must contain at least one item")
	}
	seenDocIDs := make(map[string]bool, len(labels.Labels))
	for i, item := range labels.Labels {
		if strings.TrimSpace(item.DocID) == "" {
			return fmt.Errorf("labels[%d].doc_id is required", i)
		}
		if seenDocIDs[item.DocID] {
			return fmt.Errorf("duplicate labels doc_id %q", item.DocID)
		}
		seenDocIDs[item.DocID] = true
		if strings.TrimSpace(item.ESIndex) == "" || strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.Excerpt) == "" {
			return fmt.Errorf("labels[%d] es_index, title, and excerpt are required", i)
		}
		if _, err := time.Parse(time.RFC3339, item.IngestTimestamp); err != nil {
			return fmt.Errorf("labels[%d].ingest_timestamp: %w", i, err)
		}
		if _, err := time.Parse(time.RFC3339, item.LabelledAt); err != nil {
			return fmt.Errorf("labels[%d].labelled_at: %w", i, err)
		}
		if !contains([]string{"high", "medium", "low"}, item.LabellerConfidence) {
			return fmt.Errorf("labels[%d].labeller_confidence is invalid", i)
		}
		if strings.TrimSpace(item.Labeller) == "" {
			return fmt.Errorf("labels[%d].labeller is required", i)
		}
		for _, segment := range requiredSegments {
			value, ok := item.Segments[segment]
			if !ok {
				return fmt.Errorf("labels[%d].segments missing %q", i, segment)
			}
			if !contains([]string{"strong", "partial", "none"}, value) {
				return fmt.Errorf("labels[%d].segments.%s is invalid", i, segment)
			}
		}
	}
	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
