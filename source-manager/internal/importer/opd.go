package importer

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
)

// OPDRawEntry represents a single entry from the OPD JSONL file.
type OPDRawEntry struct {
	Lemma       string          `json:"lemma"`
	WordClass   string          `json:"word_class"`
	Definitions json.RawMessage `json:"definitions"`
	Inflections json.RawMessage `json:"inflections"`
	Examples    json.RawMessage `json:"examples"`
	WordFamily  json.RawMessage `json:"word_family"`
	Media       json.RawMessage `json:"media"`
	SourceURL   string          `json:"source_url"`
	RawHTML     string          `json:"raw_html"`
}

// ImportFailure records a failed entry with its line number and reason.
type ImportFailure struct {
	Line   int    `json:"line"`
	Reason string `json:"reason"`
	Raw    string `json:"raw,omitempty"`
}

const (
	opdAttribution = "Ojibwe People's Dictionary, University of Minnesota"
	opdLicense     = "CC BY-NC-SA 4.0"
)

// ReadOPDFile reads and validates a JSONL file, returning transformed entries and failures.
func ReadOPDFile(path string) ([]models.DictionaryEntry, []ImportFailure, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	var entries []models.DictionaryEntry
	var failures []ImportFailure

	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		entry, transformErr := transformEntry(line)
		if transformErr != nil {
			failures = append(failures, ImportFailure{
				Line:   lineNum,
				Reason: transformErr.Error(),
				Raw:    line,
			})
			continue
		}

		entries = append(entries, *entry)
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return entries, failures, fmt.Errorf("read file: %w", scanErr)
	}

	return entries, failures, nil
}

func transformEntry(line string) (*models.DictionaryEntry, error) {
	var raw OPDRawEntry
	if unmarshalErr := json.Unmarshal([]byte(line), &raw); unmarshalErr != nil {
		return nil, fmt.Errorf("invalid JSON: %w", unmarshalErr)
	}

	if raw.Lemma == "" {
		return nil, errors.New("missing required field: lemma")
	}

	hash := ComputeContentHash(line)
	attribution := opdAttribution
	sourceURL := raw.SourceURL

	var wordClass, wordClassNorm *string
	if raw.WordClass != "" {
		wc := raw.WordClass
		wordClass = &wc
		wordClassNorm = &wc
	}

	return &models.DictionaryEntry{
		Lemma:               raw.Lemma,
		WordClass:           wordClass,
		WordClassNormalized: wordClassNorm,
		Definitions:         string(raw.Definitions),
		Inflections:         string(raw.Inflections),
		Examples:            string(raw.Examples),
		WordFamily:          string(raw.WordFamily),
		Media:               string(raw.Media),
		Attribution:         &attribution,
		License:             opdLicense,
		ContentHash:         &hash,
		SourceURL:           &sourceURL,
	}, nil
}

// ComputeContentHash returns the SHA-256 hex digest of canonical JSON (sorted keys).
func ComputeContentHash(jsonStr string) string {
	canonical := canonicalizeJSON(jsonStr)
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:])
}

// canonicalizeJSON parses JSON and re-serializes with sorted keys for deterministic hashing.
func canonicalizeJSON(input string) string {
	var data any
	if unmarshalErr := json.Unmarshal([]byte(input), &data); unmarshalErr != nil {
		return input
	}
	sorted := sortKeys(data)
	out, marshalErr := json.Marshal(sorted)
	if marshalErr != nil {
		return input
	}
	return string(out)
}

// sortKeys recursively sorts map keys for canonical JSON output.
func sortKeys(v any) any {
	switch val := v.(type) {
	case map[string]any:
		sorted := make(map[string]any, len(val))
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sorted[k] = sortKeys(val[k])
		}
		return sorted
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = sortKeys(item)
		}
		return result
	default:
		return v
	}
}
