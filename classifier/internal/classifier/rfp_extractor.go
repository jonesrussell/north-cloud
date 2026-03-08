package classifier

import (
	"context"
	"strconv"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// RFP extractor constants.
const (
	rfpTopicName = "rfp"

	// DocumentType values for non-solicitation procurement documents.
	rfpDocTypeNotice = "notice"
	rfpDocTypeRFI    = "rfi"
)

// noticePatterns are phrases that indicate a document is informational only (not a bid).
var noticePatterns = []string{
	"proactive disclosure",
	"notice to industry",
	"for information purposes only",
	"no response is to be submitted",
	"not a solicitation",
}

// rfiPatterns indicate a Request for Information (no bid expected).
var rfiPatterns = []string{
	"request for information",
}

// Heuristic labeled-value keys for RFP extraction.
const (
	rfpLabelReferenceNumber = "reference number:"
	rfpLabelOrganization    = "organization:"
	rfpLabelClosingDate     = "closing date:"
	rfpLabelEstimatedValue  = "estimated value:"
	rfpLabelCategory        = "category:"
	rfpLabelProvince        = "province:"
	rfpLabelCity            = "city:"
	rfpLabelCountry         = "country:"
	rfpLabelEligibility     = "eligibility:"
	rfpLabelContactName     = "contact name:"
	rfpLabelContactEmail    = "contact email:"
	rfpLabelProcurementType = "procurement type:"
)

// RFPExtractor extracts structured RFP/procurement data from raw content using
// heuristic text parsing (labeled fields like "Closing Date: value").
type RFPExtractor struct {
	logger infralogger.Logger
}

// NewRFPExtractor creates a new RFPExtractor.
func NewRFPExtractor(logger infralogger.Logger) *RFPExtractor {
	return &RFPExtractor{logger: logger}
}

// Extract attempts to extract structured RFP fields from raw content.
// Returns (nil, nil) when content is not an RFP.
func (e *RFPExtractor) Extract(
	ctx context.Context, raw *domain.RawContent, contentType string, topics []string,
) (*domain.RFPResult, error) {
	_ = ctx // reserved for future async/tracing use

	isRFPType := contentType == domain.ContentTypeRFP
	hasRFPTopic := containsTopic(topics, rfpTopicName)

	if !isRFPType && !hasRFPTopic {
		return nil, nil //nolint:nilnil // Intentional: nil result signals content is not an RFP
	}

	// Detect non-solicitation document type before extraction.
	docType := detectRFPDocumentType(raw.Title + " " + raw.RawText)

	// Tier 1: Heuristic extraction (labeled fields).
	if result := e.extractHeuristic(raw.RawText); result != nil {
		result.DocumentType = docType
		e.logger.Debug("RFP extracted via heuristic",
			infralogger.String("content_id", raw.ID),
			infralogger.String("reference_number", result.ReferenceNumber),
			infralogger.String("document_type", result.DocumentType),
		)
		return result, nil
	}

	// Even without labeled fields, record the document type if detected.
	if docType != "" {
		return &domain.RFPResult{
			ExtractionMethod: extractionMethodHeuristic,
			DocumentType:     docType,
		}, nil
	}

	return nil, nil //nolint:nilnil // Intentional: nil result signals no RFP data found
}

// extractHeuristic performs text-based RFP extraction by looking for
// labeled fields like "Reference Number: <value>", "Closing Date: <value>", etc.
// Returns nil if no recognizable RFP patterns are found.
func (e *RFPExtractor) extractHeuristic(rawText string) *domain.RFPResult {
	lowerText := strings.ToLower(rawText)

	refNumber := extractLabeledValue(rawText, lowerText, rfpLabelReferenceNumber)
	org := extractLabeledValue(rawText, lowerText, rfpLabelOrganization)
	closingDate := extractLabeledValue(rawText, lowerText, rfpLabelClosingDate)
	estimatedValue := extractLabeledValue(rawText, lowerText, rfpLabelEstimatedValue)
	category := extractLabeledValue(rawText, lowerText, rfpLabelCategory)
	province := extractLabeledValue(rawText, lowerText, rfpLabelProvince)
	city := extractLabeledValue(rawText, lowerText, rfpLabelCity)
	country := extractLabeledValue(rawText, lowerText, rfpLabelCountry)
	eligibility := extractLabeledValue(rawText, lowerText, rfpLabelEligibility)
	contactName := extractLabeledValue(rawText, lowerText, rfpLabelContactName)
	contactEmail := extractLabeledValue(rawText, lowerText, rfpLabelContactEmail)
	procurementType := extractLabeledValue(rawText, lowerText, rfpLabelProcurementType)

	// Require at least one recognizable RFP field beyond just a category
	if refNumber == "" && org == "" && closingDate == "" {
		return nil
	}

	result := &domain.RFPResult{
		ExtractionMethod: extractionMethodHeuristic,
		ReferenceNumber:  refNumber,
		OrganizationName: org,
		ClosingDate:      closingDate,
		Province:         province,
		City:             city,
		Country:          country,
		Eligibility:      eligibility,
		ContactName:      contactName,
		ContactEmail:     contactEmail,
		ProcurementType:  procurementType,
	}

	if category != "" {
		result.Categories = []string{category}
	}

	result.BudgetMin, result.BudgetMax, result.BudgetCurrency = parseBudgetValue(estimatedValue)

	return result
}

// parseBudgetValue extracts numeric budget from strings like "$500,000" or "$100,000 - $500,000".
// Returns (nil, nil, "") if no recognizable budget is found.
func parseBudgetValue(value string) (budgetMin, budgetMax *float64, currency string) {
	if value == "" {
		return nil, nil, ""
	}

	currency = "CAD" // Default for Canadian sources
	if strings.Contains(value, "USD") || strings.Contains(value, "US$") {
		currency = "USD"
	}

	// Strip currency symbols and commas for parsing
	cleaned := strings.NewReplacer("$", "", ",", "", "CAD", "", "USD", "", "US", "").Replace(value)
	cleaned = strings.TrimSpace(cleaned)

	// Try range format: "100000 - 500000"
	if parts := strings.SplitN(cleaned, "-", 2); len(parts) == 2 { //nolint:mnd // split into exactly 2 parts for range parsing
		minStr := strings.TrimSpace(parts[0])
		maxStr := strings.TrimSpace(parts[1])
		minVal := parseFloat(minStr)
		maxVal := parseFloat(maxStr)
		if minVal != nil || maxVal != nil {
			return minVal, maxVal, currency
		}
	}

	// Single value — treat as both min and max
	val := parseFloat(cleaned)
	if val != nil {
		return val, val, currency
	}

	return nil, nil, ""
}

// detectRFPDocumentType checks combined title+body text for non-solicitation signals.
// Returns "notice", "rfi", or "" (normal bid/solicitation).
func detectRFPDocumentType(text string) string {
	lower := strings.ToLower(text)
	for _, pattern := range noticePatterns {
		if strings.Contains(lower, pattern) {
			return rfpDocTypeNotice
		}
	}
	for _, pattern := range rfiPatterns {
		if strings.Contains(lower, pattern) {
			return rfpDocTypeRFI
		}
	}
	return ""
}

// parseFloat extracts a positive float64 from a pre-cleaned numeric string.
// Strips any remaining non-numeric characters (except '.' and '-') before parsing.
// Returns nil if the string is empty, non-numeric, or the result is not positive.
func parseFloat(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Strip any remaining non-numeric characters (spaces, currency remnants)
	cleaned := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || r == '.' {
			return r
		}
		return -1
	}, s)
	if cleaned == "" {
		return nil
	}
	val, err := strconv.ParseFloat(cleaned, 64)
	if err != nil || val <= 0 {
		return nil
	}
	return &val
}
