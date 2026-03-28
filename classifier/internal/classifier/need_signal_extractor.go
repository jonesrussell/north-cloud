package classifier

import (
	"context"
	"regexp"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// Signal type constants for need signal classification.
const (
	SignalTypeOutdatedWebsite = "outdated_website"
	SignalTypeFundingWin      = "funding_win"
	SignalTypeJobPosting      = "job_posting"
	SignalTypeNewProgram      = "new_program"
	SignalTypeTechMigration   = "tech_migration"
)

// needSignalConfidence is the default confidence for keyword-based need signal extraction.
const needSignalConfidence = 0.80

// signalCategoryKeywords maps each signal type to its detection keywords.
var signalCategoryKeywords = map[string][]string{
	SignalTypeOutdatedWebsite: {
		"drupal 7", "legacy website", "outdated website", "website redesign",
		"site redesign", "website overhaul", "joomla", "wordpress 4",
		"end of life", "eol", "unsupported platform",
	},
	SignalTypeFundingWin: {
		"funding announcement", "grant funding", "receives funding",
		"awarded grant", "digital capacity", "capital funding",
		"infrastructure funding", "received grant", "funding approved",
	},
	SignalTypeJobPosting: {
		"web developer", "frontend developer", "full stack developer",
		"seeking a developer", "hiring a developer", "website development",
		"developer position",
	},
	SignalTypeNewProgram: {
		"new program launch", "program expansion", "service expansion",
		"digital strategy", "online presence", "digital transformation",
	},
	SignalTypeTechMigration: {
		"site migration", "website migration", "platform migration",
		"wordpress migration", "joomla migration", "technology modernization",
		"website modernization", "content management system",
	},
}

// titleDelimiters are used to extract organization names from titles.
var titleDelimiters = []string{
	" - ", " | ", ": ", " announces ", " receives ", " awarded ", " launches ",
}

// emailRegex matches email addresses in content.
var emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

// NeedSignalExtractor extracts structured data from content classified as need_signal.
type NeedSignalExtractor struct {
	logger infralogger.Logger
}

// NewNeedSignalExtractor creates a new NeedSignalExtractor.
func NewNeedSignalExtractor(logger infralogger.Logger) *NeedSignalExtractor {
	return &NeedSignalExtractor{logger: logger}
}

// Extract attempts to extract structured need signal fields from raw content.
// Returns (nil, nil) when content is not a need signal.
func (e *NeedSignalExtractor) Extract(
	ctx context.Context, raw *domain.RawContent, contentType string, _ []string,
) (*domain.NeedSignalResult, error) {
	_ = ctx // reserved for future async/tracing use

	if contentType != domain.ContentTypeNeedSignal {
		return nil, nil //nolint:nilnil // Intentional: nil result signals content is not a need signal
	}

	combinedText := strings.ToLower(raw.Title + " " + raw.RawText)

	signalType := detectSignalType(combinedText)
	orgName := extractOrgName(raw.Title)
	contactEmail := extractContactEmail(raw.RawText)
	keywords := collectMatchedKeywords(combinedText, signalType)

	result := &domain.NeedSignalResult{
		SignalType:       signalType,
		OrganizationName: orgName,
		ContactEmail:     contactEmail,
		Keywords:         keywords,
		Confidence:       needSignalConfidence,
		SourceURL:        raw.URL,
	}

	e.logger.Debug("Need signal extracted",
		infralogger.String("content_id", raw.ID),
		infralogger.String("signal_type", signalType),
		infralogger.String("organization", orgName),
	)

	return result, nil
}

// detectSignalType counts keyword matches per category and returns the highest.
// Returns "unknown" when no keywords match.
func detectSignalType(text string) string {
	bestType := "unknown"
	bestCount := 0

	for signalType, keywords := range signalCategoryKeywords {
		count := 0
		for _, kw := range keywords {
			if strings.Contains(text, kw) {
				count++
			}
		}

		if count > bestCount {
			bestCount = count
			bestType = signalType
		}
	}

	return bestType
}

// extractOrgName extracts an organization name from the title using common delimiters.
func extractOrgName(title string) string {
	for _, delim := range titleDelimiters {
		lowerTitle := strings.ToLower(title)
		lowerDelim := strings.ToLower(delim)

		idx := strings.Index(lowerTitle, lowerDelim)
		if idx > 0 {
			return strings.TrimSpace(title[:idx])
		}
	}

	return title
}

// extractContactEmail finds the first email address in the text.
func extractContactEmail(text string) string {
	return emailRegex.FindString(text)
}

// collectMatchedKeywords returns keywords that matched for the given signal type.
func collectMatchedKeywords(text, signalType string) []string {
	keywords, ok := signalCategoryKeywords[signalType]
	if !ok {
		return nil
	}

	matched := make([]string, 0, len(keywords))

	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			matched = append(matched, kw)
		}
	}

	return matched
}
