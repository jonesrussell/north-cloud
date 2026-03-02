package classifier

import (
	"context"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// RFP extractor constants.
const (
	rfpTopicName = "rfp"
)

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

	// Tier 1: Heuristic extraction (labeled fields).
	if result := e.extractHeuristic(raw.RawText); result != nil {
		e.logger.Debug("RFP extracted via heuristic",
			infralogger.String("content_id", raw.ID),
			infralogger.String("reference_number", result.ReferenceNumber),
		)
		return result, nil
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

// decimalBase is the base used for decimal digit conversion.
const decimalBase = 10

// parseFloat parses a string to *float64, returning nil on failure.
func parseFloat(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var val float64
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			val = val*decimalBase + float64(ch-'0')
		} else if ch != '.' {
			// Non-numeric, non-dot character — bail
			if val > 0 {
				return &val
			}
			return nil
		}
	}
	if val > 0 {
		return &val
	}
	return nil
}
