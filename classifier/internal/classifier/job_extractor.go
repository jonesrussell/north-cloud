package classifier

import (
	"context"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/classifier/jsonld"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// Job extractor constants.
const (
	jobTopicName           = "jobs"
	schemaOrgTypeJobPost   = "JobPosting"
	locationSeparator      = ", "
	heuristicCompanyLabel  = "company:"
	heuristicLocationLabel = "location:"
)

// Heuristic section header labels for job qualifications (case-insensitive).
var jobQualificationHeaders = []string{
	"requirements:",
	"qualifications:",
}

// employmentTypeMap normalizes Schema.org employment type values to lowercase.
var employmentTypeMap = map[string]string{
	"FULL_TIME":  "full_time",
	"PART_TIME":  "part_time",
	"CONTRACT":   "contract",
	"TEMPORARY":  "temporary",
	"INTERN":     "internship",
	"INTERNSHIP": "internship",
}

// JobExtractor extracts structured job posting data from raw content using
// Schema.org JSON-LD (tier 1) or heuristic text parsing (tier 2).
type JobExtractor struct {
	logger infralogger.Logger
}

// NewJobExtractor creates a new JobExtractor.
func NewJobExtractor(logger infralogger.Logger) *JobExtractor {
	return &JobExtractor{logger: logger}
}

// Extract attempts to extract structured job fields from raw content.
// Returns (nil, nil) when content is not a job posting.
func (e *JobExtractor) Extract(
	ctx context.Context, raw *domain.RawContent, contentType string, topics []string,
) (*domain.JobResult, error) {
	_ = ctx // reserved for future async/tracing use

	isJobType := contentType == domain.ContentTypeJob
	hasJobTopic := containsTopic(topics, jobTopicName)

	if !isJobType && !hasJobTopic {
		return nil, nil //nolint:nilnil // Intentional: nil result signals content is not a job
	}

	// Tier 1: Schema.org JSON-LD extraction.
	if result := e.extractSchemaOrg(raw.RawHTML); result != nil {
		e.logger.Debug("job extracted via Schema.org",
			infralogger.String("content_id", raw.ID),
			infralogger.String("title", result.Title),
		)

		return result, nil
	}

	// Tier 2: Heuristic extraction (only when topic matched or content type is job).
	if hasJobTopic || isJobType {
		if result := e.extractHeuristic(raw.RawText); result != nil {
			e.logger.Debug("job extracted via heuristic",
				infralogger.String("content_id", raw.ID),
			)

			return result, nil
		}
	}

	return nil, nil //nolint:nilnil // Intentional: nil result signals no job data found
}

// extractSchemaOrg tries to extract a JobPosting from JSON-LD blocks in HTML.
// Returns nil if no valid JobPosting block is found.
func (e *JobExtractor) extractSchemaOrg(html string) *domain.JobResult {
	blocks := jsonld.Extract(html, nil)
	job := jsonld.FindByType(blocks, schemaOrgTypeJobPost)

	if job == nil {
		return nil
	}

	result := &domain.JobResult{
		ExtractionMethod: extractionMethodSchemaOrg,
		Title:            jsonld.StringVal(job, "title"),
		Company:          jsonld.NestedStringVal(job, "hiringOrganization", "name"),
		Location:         extractJobLocation(job),
		EmploymentType:   normalizeEmploymentType(jsonld.StringVal(job, "employmentType")),
		PostedDate:       jsonld.StringVal(job, "datePosted"),
		ExpiresDate:      jsonld.StringVal(job, "validThrough"),
		Description:      jsonld.StringVal(job, "description"),
		Industry:         jsonld.StringVal(job, "industry"),
		Qualifications:   jsonld.StringVal(job, "qualifications"),
		Benefits:         jsonld.StringVal(job, "jobBenefits"),
	}

	extractSalary(job, result)

	return result
}

// extractJobLocation builds a normalized "City, Region" string from
// the jobLocation.address nested structure.
func extractJobLocation(job map[string]any) string {
	locMap, ok := job["jobLocation"].(map[string]any)
	if !ok {
		return ""
	}

	addrMap, ok := locMap["address"].(map[string]any)
	if !ok {
		return ""
	}

	city := jsonld.StringVal(addrMap, "addressLocality")
	region := jsonld.StringVal(addrMap, "addressRegion")

	if city != "" && region != "" {
		return city + locationSeparator + region
	}

	if city != "" {
		return city
	}

	return region
}

// extractSalary populates salary fields from a baseSalary nested structure.
func extractSalary(job map[string]any, result *domain.JobResult) {
	salaryMap, ok := job["baseSalary"].(map[string]any)
	if !ok {
		return
	}

	result.SalaryCurrency = jsonld.StringVal(salaryMap, "currency")

	valueMap, ok := salaryMap["value"].(map[string]any)
	if !ok {
		return
	}

	result.SalaryMin = jsonld.FloatVal(valueMap, "minValue")
	result.SalaryMax = jsonld.FloatVal(valueMap, "maxValue")
}

// normalizeEmploymentType converts Schema.org employment type values
// to lowercase normalized forms. Unknown values are lowercased as-is.
func normalizeEmploymentType(raw string) string {
	if raw == "" {
		return ""
	}

	if normalized, ok := employmentTypeMap[raw]; ok {
		return normalized
	}

	return strings.ToLower(raw)
}

// extractHeuristic performs text-based job extraction by looking for
// patterns like "Company: <value>" and "Location: <value>".
// Returns nil if no recognizable job patterns are found.
func (e *JobExtractor) extractHeuristic(rawText string) *domain.JobResult {
	lowerText := strings.ToLower(rawText)

	company := extractLabeledValue(rawText, lowerText, heuristicCompanyLabel)
	location := extractLabeledValue(rawText, lowerText, heuristicLocationLabel)
	qualifications := extractJobQualificationSection(rawText, lowerText)

	if company == "" && location == "" && qualifications == "" {
		return nil
	}

	return &domain.JobResult{
		ExtractionMethod: extractionMethodHeuristic,
		Company:          company,
		Location:         location,
		Qualifications:   qualifications,
	}
}

// extractLabeledValue finds a "Label: value" line in text and returns
// the trimmed value portion. The label is matched case-insensitively.
func extractLabeledValue(rawText, lowerText, label string) string {
	idx := strings.Index(lowerText, label)
	if idx < 0 {
		return ""
	}

	// Start after the label.
	valueStart := idx + len(label)

	// Find the end of the line.
	lineEnd := strings.Index(rawText[valueStart:], "\n")
	if lineEnd < 0 {
		return strings.TrimSpace(rawText[valueStart:])
	}

	return strings.TrimSpace(rawText[valueStart : valueStart+lineEnd])
}

// extractJobQualificationSection finds and returns text from a
// Requirements or Qualifications section.
func extractJobQualificationSection(rawText, lowerText string) string {
	headerIdx := findSectionHeader(lowerText, jobQualificationHeaders)
	if headerIdx < 0 {
		return ""
	}

	lineStart := strings.Index(rawText[headerIdx:], "\n")
	if lineStart < 0 {
		return ""
	}

	sectionStart := headerIdx + lineStart + 1
	sectionEnd := findNextSectionEnd(rawText, sectionStart)
	section := strings.TrimSpace(rawText[sectionStart:sectionEnd])

	return section
}
