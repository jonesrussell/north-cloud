package parser

import (
	"bufio"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	infrasignal "github.com/jonesrussell/north-cloud/infrastructure/signal"
	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/domain"
)

// bomLength is the byte length of a UTF-8 BOM (EF BB BF).
const bomLength = 3

// CSV column names from the CanadaBuys feed.
const (
	colTitle              = "title-titre-eng"
	colReferenceNumber    = "referenceNumber-numeroReference"
	colSolicitationNumber = "solicitationNumber-numeroSollicitation"
	colAmendmentNumber    = "amendmentNumber-numeroModification"
	colPublicationDate    = "publicationDate-datePublication"
	colClosingDate        = "tenderClosingDate-appelOffresDateCloture"
	colAmendmentDate      = "amendmentDate-dateModification"
	colTenderStatus       = "tenderStatus-appelOffresStatut-eng"
	colGSIN               = "gsin-nibs"
	colUNSPSC             = "unspsc"
	colProcurementCat     = "procurementCategory-categorieApprovisionnement"
	colRegionsOfDelivery  = "regionsOfDelivery-regionsLivraison-eng"
	colOrgName            = "contractingEntityName-nomEntitContractante-eng"
	colCity               = "contractingEntityAddressCity-entiteContractanteAdresseVille-eng"
	colContactName        = "contactInfoName-informationsContactNom"
	colContactEmail       = "contactInfoEmail-informationsContactCourriel"
	colNoticeURL          = "noticeURL-URLavis-eng"
	colDescription        = "tenderDescription-descriptionAppelOffres-eng"
)

// canadaBuysTenderBaseURL is the base URL for CanadaBuys tender notice pages.
const canadaBuysTenderBaseURL = "https://canadabuys.canada.ca/en/tender-opportunities/tender-notice/"

// Hardcoded values for all CanadaBuys documents.
const (
	cbSourceName       = "CanadaBuys"
	cbContentType      = "rfp"
	cbQualityScore     = 80
	cbCountry          = "CA"
	cbBudgetCurrency   = "CAD"
	cbExtractionMethod = "csv_feed"
	cbSnippetMaxLen    = 200
)

// provinceMap maps region names (lowercase) to 2-letter province codes.
var provinceMap = map[string]string{
	"alberta":                         "ab",
	"british columbia":                "bc",
	"manitoba":                        "mb",
	"new brunswick":                   "nb",
	"newfoundland and labrador":       "nl",
	"nova scotia":                     "ns",
	"ontario":                         "on",
	"prince edward island":            "pe",
	"quebec":                          "qc",
	"saskatchewan":                    "sk",
	"northwest territories":           "nt",
	"nunavut":                         "nu",
	"yukon":                           "yt",
	"national capital region (ncr)":   "on",
	"région de la capitale nationale": "on",
}

// CanadaBuysParser parses CanadaBuys CSV feeds into RFPDocuments.
type CanadaBuysParser struct{}

// NewCanadaBuysParser creates a new CanadaBuys CSV parser.
func NewCanadaBuysParser() *CanadaBuysParser {
	return &CanadaBuysParser{}
}

// SourceName returns the canonical source identifier.
func (p *CanadaBuysParser) SourceName() string {
	return cbSourceName
}

// Parse reads a CanadaBuys CSV from r and returns parsed RFPDocuments keyed by document ID.
// Only rows with tender status "Open" are included.
func (p *CanadaBuysParser) Parse(r io.Reader) (map[string]domain.RFPDocument, []error, error) {
	docs, errs := ParseCanadaBuysCSV(r)
	if len(errs) > 0 && len(docs) == 0 {
		return nil, errs, errs[0]
	}

	result := make(map[string]domain.RFPDocument, len(docs))
	for i := range docs {
		result[CanadaBuysDocumentID(docs[i])] = docs[i]
	}

	return result, errs, nil
}

// ParseCanadaBuysCSV reads a CanadaBuys CSV from r and returns parsed RFPDocuments.
// Only rows with tender status "Open" are included. Parse errors for
// individual rows are collected and returned alongside any successfully
// parsed documents.
func ParseCanadaBuysCSV(r io.Reader) ([]domain.RFPDocument, []error) {
	r = stripBOM(r)
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

// CanadaBuysDocumentID produces a deterministic SHA-256 hex string from the
// reference number and amendment number of an RFP document.
func CanadaBuysDocumentID(doc domain.RFPDocument) string {
	input := doc.RFP.ReferenceNumber + ":" + doc.RFP.AmendmentNumber
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

// buildColumnIndex maps column header names to their indices.
func buildColumnIndex(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, col := range header {
		idx[strings.TrimSpace(col)] = i
	}
	return idx
}

// stripBOM returns a reader that skips a leading UTF-8 BOM (EF BB BF)
// if present. CanadaBuys CSV feeds include a BOM that confuses Go's csv.Reader.
func stripBOM(r io.Reader) io.Reader {
	br := bufio.NewReader(r)
	bom, err := br.Peek(bomLength)
	if err == nil && len(bom) >= bomLength && bom[0] == 0xEF && bom[1] == 0xBB && bom[2] == 0xBF {
		_, _ = br.Discard(bomLength)
	}
	return br
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
		return domain.RFPDocument{}, errors.New("missing reference number")
	}

	gsin := getField(record, colIndex, colGSIN)
	unspsc := getField(record, colIndex, colUNSPSC)
	description := getField(record, colIndex, colDescription)
	region := getField(record, colIndex, colRegionsOfDelivery)
	procurementCat := getField(record, colIndex, colProcurementCat)
	noticeURL := getField(record, colIndex, colNoticeURL)
	if noticeURL == "" {
		noticeURL = canadaBuysTenderBaseURL + refNumber
	}

	orgName := getField(record, colIndex, colOrgName)
	contactEmail := getField(record, colIndex, colContactEmail)
	// Resolve canonical slug via shared attribution fallback. Ignore the error:
	// CanadaBuys always supplies an organization_name, so a miss means the row
	// was truncated and the validation sweep will flag it.
	orgNormalized, _ := infrasignal.Resolve(orgName, contactEmail, noticeURL)

	return domain.RFPDocument{
		Title:        title,
		URL:          noticeURL,
		SourceName:   cbSourceName,
		ContentType:  cbContentType,
		QualityScore: cbQualityScore,
		Snippet:      truncate(description, cbSnippetMaxLen),
		RawText:      description,
		Topics:       deriveTopics(gsin, unspsc),
		CrawledAt:    crawledAt,
		RFP: domain.RFP{
			ExtractionMethod:           cbExtractionMethod,
			Title:                      title,
			ReferenceNumber:            refNumber,
			OrganizationName:           orgName,
			OrganizationNameNormalized: orgNormalized,
			Description:                description,
			PublishedDate:              getField(record, colIndex, colPublicationDate),
			ClosingDate:                getField(record, colIndex, colClosingDate),
			AmendmentDate:              getField(record, colIndex, colAmendmentDate),
			AmendmentNumber:            getField(record, colIndex, colAmendmentNumber),
			BudgetCurrency:             cbBudgetCurrency,
			ProcurementType:            normalizeProcurementType(procurementCat),
			Categories:                 deriveCategories(gsin, unspsc),
			Province:                   NormalizeProvince(region),
			City:                       getField(record, colIndex, colCity),
			Country:                    cbCountry,
			SourceURL:                  noticeURL,
			ContactName:                getField(record, colIndex, colContactName),
			ContactEmail:               contactEmail,
			GSIN:                       gsin,
			UNSPSC:                     unspsc,
			TenderStatus:               getField(record, colIndex, colTenderStatus),
		},
	}, nil
}

// NormalizeProvince maps a region name to a 2-letter province code.
// It strips leading "*" characters and is case-insensitive.
// Returns an empty string for unrecognized regions.
func NormalizeProvince(region string) string {
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

// DeriveCategories produces a list of category tags based on GSIN and UNSPSC codes.
func DeriveCategories(gsin, unspsc string) []string {
	return deriveCategories(gsin, unspsc)
}

func deriveCategories(gsin, unspsc string) []string {
	var categories []string

	if IsITRelated(gsin, unspsc) {
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

// IsITRelated returns true if the GSIN or UNSPSC codes indicate an
// IT-related procurement.
func IsITRelated(gsin, unspsc string) bool {
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
	if IsITRelated(gsin, unspsc) {
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
