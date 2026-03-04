package ingestor

import (
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/domain"
)

// CSV column names from the CanadaBuys feed.
const (
	colTitle             = "title-titre-eng"
	colReferenceNumber     = "referenceNumber-numeroReference"
	colSolicitationNumber = "solicitationNumber-numeroSollicitation"
	colAmendmentNumber    = "amendmentNumber-numeroModification"
	colPublicationDate   = "publicationDate-datePublication"
	colClosingDate       = "tenderClosingDate-appelOffresDateCloture"
	colAmendmentDate     = "amendmentDate-dateModification"
	colTenderStatus      = "tenderStatus-appelOffresStatut-eng"
	colGSIN              = "gsin-nibs"
	colUNSPSC            = "unspsc"
	colProcurementCat    = "procurementCategory-categorieApprovisionnement"
	colRegionsOfDelivery = "regionsOfDelivery-regionsLivraison-eng"
	colOrgName           = "contractingEntityName-nomEntitContractante-eng"
	colCity              = "contractingEntityAddressCity-entiteContractanteAdresseVille-eng"
	colContactName       = "contactInfoName-informationsContactNom"
	colContactEmail      = "contactInfoEmail-informationsContactCourriel"
	colNoticeURL         = "noticeURL-URLavis-eng"
	colDescription       = "tenderDescription-descriptionAppelOffres-eng"
)

// Hardcoded values for all CanadaBuys documents.
const (
	sourceName       = "CanadaBuys"
	contentType      = "rfp"
	qualityScore     = 80
	country          = "CA"
	budgetCurrency   = "CAD"
	extractionMethod = "csv_feed"
	snippetMaxLen    = 200
)

// provinceMap maps region names (lowercase) to 2-letter province codes.
var provinceMap = map[string]string{
	"alberta":                          "ab",
	"british columbia":                 "bc",
	"manitoba":                         "mb",
	"new brunswick":                    "nb",
	"newfoundland and labrador":        "nl",
	"nova scotia":                      "ns",
	"ontario":                          "on",
	"prince edward island":             "pe",
	"quebec":                           "qc",
	"saskatchewan":                     "sk",
	"northwest territories":            "nt",
	"nunavut":                          "nu",
	"yukon":                            "yt",
	"national capital region (ncr)":    "on",
	"région de la capitale nationale":  "on",
}

// ParseCSV reads a CanadaBuys CSV from r and returns parsed RFPDocuments.
// Only rows with tender status "Open" are included. Parse errors for
// individual rows are collected and returned alongside any successfully
// parsed documents.
func ParseCSV(r io.Reader) ([]domain.RFPDocument, []error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return nil, []error{fmt.Errorf("read CSV header: %w", err)}
	}

	colIndex := buildColumnIndex(header)

	var docs []domain.RFPDocument
	var errs []error

	now := time.Now().UTC().Format(time.RFC3339)

	for rowNum := 2; ; rowNum++ {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			errs = append(errs, fmt.Errorf("row %d: %w", rowNum, readErr))
			continue
		}

		status := getField(record, colIndex, colTenderStatus)
		if status != "Open" {
			continue
		}

		doc, buildErr := buildDocument(record, colIndex, now)
		if buildErr != nil {
			errs = append(errs, fmt.Errorf("row %d: %w", rowNum, buildErr))
			continue
		}

		docs = append(docs, doc)
	}

	return docs, errs
}

// DocumentID produces a deterministic SHA-256 hex string from the
// reference number and amendment number of an RFP document.
func DocumentID(doc domain.RFPDocument) string {
	input := doc.RFP.ReferenceNumber + ":" + doc.RFP.AmendmentNumber
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash)
}

// buildColumnIndex maps column header names to their indices.
func buildColumnIndex(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, col := range header {
		idx[strings.TrimSpace(col)] = i
	}
	return idx
}

// getField safely retrieves a field value by column name.
func getField(record []string, colIndex map[string]int, column string) string {
	i, ok := colIndex[column]
	if !ok || i >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[i])
}

// buildDocument constructs an RFPDocument from a single CSV row.
func buildDocument(record []string, colIndex map[string]int, crawledAt string) (domain.RFPDocument, error) {
	title := getField(record, colIndex, colTitle)
	refNumber := getField(record, colIndex, colReferenceNumber)
	if refNumber == "" {
		refNumber = getField(record, colIndex, colSolicitationNumber)
	}
	if refNumber == "" {
		return domain.RFPDocument{}, fmt.Errorf("missing reference number")
	}

	gsin := getField(record, colIndex, colGSIN)
	unspsc := getField(record, colIndex, colUNSPSC)
	description := getField(record, colIndex, colDescription)
	region := getField(record, colIndex, colRegionsOfDelivery)
	procurementCat := getField(record, colIndex, colProcurementCat)
	noticeURL := getField(record, colIndex, colNoticeURL)

	return domain.RFPDocument{
		Title:        title,
		URL:          noticeURL,
		SourceName:   sourceName,
		ContentType:  contentType,
		QualityScore: qualityScore,
		Snippet:      truncate(description, snippetMaxLen),
		Topics:       deriveTopics(gsin, unspsc),
		CrawledAt:    crawledAt,
		RFP: domain.RFP{
			ExtractionMethod: extractionMethod,
			Title:            title,
			ReferenceNumber:  refNumber,
			OrganizationName: getField(record, colIndex, colOrgName),
			Description:      description,
			PublishedDate:    getField(record, colIndex, colPublicationDate),
			ClosingDate:      getField(record, colIndex, colClosingDate),
			AmendmentDate:    getField(record, colIndex, colAmendmentDate),
			AmendmentNumber:  getField(record, colIndex, colAmendmentNumber),
			BudgetCurrency:   budgetCurrency,
			ProcurementType:  normalizeProcurementType(procurementCat),
			Categories:       deriveCategories(gsin, unspsc),
			Province:         normalizeProvince(region),
			City:             getField(record, colIndex, colCity),
			Country:          country,
			SourceURL:        noticeURL,
			ContactName:      getField(record, colIndex, colContactName),
			ContactEmail:     getField(record, colIndex, colContactEmail),
			GSIN:             gsin,
			UNSPSC:           unspsc,
			TenderStatus:     getField(record, colIndex, colTenderStatus),
		},
	}, nil
}

// normalizeProvince maps a region name to a 2-letter province code.
// It strips leading "*" characters and is case-insensitive.
// Returns an empty string for unrecognized regions.
func normalizeProvince(region string) string {
	cleaned := strings.TrimLeft(region, "*")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return ""
	}

	code, ok := provinceMap[strings.ToLower(cleaned)]
	if !ok {
		return ""
	}
	return code
}

// normalizeProcurementType maps CanadaBuys procurement category codes to
// human-readable types.
func normalizeProcurementType(category string) string {
	cleaned := strings.TrimLeft(category, "*")
	cleaned = strings.TrimSpace(cleaned)

	switch strings.ToUpper(cleaned) {
	case "GD":
		return "goods"
	case "SV":
		return "services"
	case "CN":
		return "construction"
	default:
		return "mixed"
	}
}

// deriveCategories produces a list of category tags based on GSIN and UNSPSC codes.
func deriveCategories(gsin, unspsc string) []string {
	var categories []string

	if isITRelated(gsin, unspsc) {
		categories = append(categories, "it")
	}

	if strings.HasPrefix(unspsc, "*4323") {
		categories = append(categories, "software")
	} else if strings.HasPrefix(unspsc, "*8111") || strings.HasPrefix(unspsc, "*8112") {
		categories = append(categories, "it-services")
	} else if strings.HasPrefix(unspsc, "*43") || strings.HasPrefix(unspsc, "*81") {
		categories = append(categories, "it")
	}

	return deduplicate(categories)
}

// isITRelated returns true if the GSIN or UNSPSC codes indicate an
// IT-related procurement.
func isITRelated(gsin, unspsc string) bool {
	if strings.HasPrefix(gsin, "*D") {
		return true
	}
	if strings.HasPrefix(unspsc, "*43") || strings.HasPrefix(unspsc, "*81") {
		return true
	}
	return false
}

// deriveTopics produces a list of topic tags. All CanadaBuys documents
// get "politics"; IT-related ones also get "technology".
func deriveTopics(gsin, unspsc string) []string {
	topics := []string{"politics"}
	if isITRelated(gsin, unspsc) {
		topics = append(topics, "technology")
	}
	return topics
}

// truncate returns the first maxLen characters of s, or s itself if shorter.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

// deduplicate removes duplicate strings from a slice while preserving order.
func deduplicate(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if _, exists := seen[item]; !exists {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}
