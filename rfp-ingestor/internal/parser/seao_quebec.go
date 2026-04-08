package parser

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/domain"
)

// SEAO constants.
const (
	seaoSourceName       = "SEAO"
	seaoContentType      = "rfp"
	seaoQualityScore     = 75
	seaoCountry          = "CA"
	seaoBudgetCurrency   = "CAD"
	seaoExtractionMethod = "json_feed"
	seaoSnippetMaxLen    = 200
	seaoBaseURL          = "https://seao.gouv.qc.ca"

	classificationSchemeUNSPSC = "UNSPSC"
)

// SEAOParser parses SEAO Quebec OCDS JSON feeds into RFPDocuments.
type SEAOParser struct{}

// NewSEAOParser creates a new SEAO Quebec JSON parser.
func NewSEAOParser() *SEAOParser {
	return &SEAOParser{}
}

// SourceName returns the canonical source identifier.
func (p *SEAOParser) SourceName() string {
	return seaoSourceName
}

// Parse reads an SEAO OCDS JSON feed from r and returns active tenders
// keyed by document ID.
func (p *SEAOParser) Parse(r io.Reader) (map[string]domain.RFPDocument, error) {
	var feed seaoFeed
	if err := json.NewDecoder(r).Decode(&feed); err != nil {
		return nil, fmt.Errorf("decode SEAO JSON: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result := make(map[string]domain.RFPDocument)

	for i := range feed.Releases {
		release := &feed.Releases[i]
		if !isActiveTender(release) {
			continue
		}

		doc := buildSEAODocument(release, now)
		docID := seaoDocumentID(release.OCID, release.Tender.ID)
		result[docID] = doc
	}

	return result, nil
}

// isActiveTender returns true if the release represents an active tender.
func isActiveTender(r *seaoRelease) bool {
	if r.Tender.Status != "active" {
		return false
	}

	for _, tag := range r.Tag {
		if tag == "tender" || tag == "tenderUpdate" {
			return true
		}
	}

	return false
}

func buildSEAODocument(r *seaoRelease, crawledAt string) domain.RFPDocument {
	title := r.Tender.Title
	description := buildSEAODescription(r)
	org := r.Buyer.Name
	province := extractBuyerProvince(r.Parties)
	city := extractBuyerCity(r.Parties)
	unspsc := extractPrimaryUNSPSC(r.Tender.Items)
	categories := deriveSEAOCategories(r.Tender.Items)
	closingDate := r.Tender.TenderPeriod.EndDate
	procurementType := r.Tender.MainProcurementCategory
	if procurementType == "" {
		procurementType = "mixed"
	}

	sourceURL := seaoBaseURL

	topics := []string{"politics"}
	if hasSEAOITClassification(r.Tender.Items) {
		topics = append(topics, "technology")
	}

	return domain.RFPDocument{
		Title:        title,
		URL:          sourceURL,
		SourceName:   seaoSourceName,
		ContentType:  seaoContentType,
		QualityScore: seaoQualityScore,
		Snippet:      truncate(description, seaoSnippetMaxLen),
		RawText:      description,
		Topics:       topics,
		CrawledAt:    crawledAt,
		RFP: domain.RFP{
			ExtractionMethod: seaoExtractionMethod,
			Title:            title,
			ReferenceNumber:  r.Tender.ID,
			OrganizationName: org,
			Description:      description,
			PublishedDate:    r.Date,
			ClosingDate:      closingDate,
			BudgetCurrency:   seaoBudgetCurrency,
			ProcurementType:  procurementType,
			Categories:       categories,
			Province:         province,
			City:             city,
			Country:          seaoCountry,
			SourceURL:        sourceURL,
			UNSPSC:           unspsc,
			TenderStatus:     r.Tender.Status,
		},
	}
}

// seaoDocumentID produces a deterministic SHA-256 hex string from OCID and tender ID.
func seaoDocumentID(ocid, tenderID string) string {
	input := ocid + ":" + tenderID
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

// buildSEAODescription constructs a description from tender items.
func buildSEAODescription(r *seaoRelease) string {
	var parts []string
	parts = append(parts, r.Tender.Title)

	for _, item := range r.Tender.Items {
		if item.Description != "" {
			parts = append(parts, item.Description)
		}
		if item.Classification.Description != "" {
			parts = append(parts, item.Classification.Description)
		}
	}

	if r.Tender.ProcurementMethodDetails != "" {
		parts = append(parts, r.Tender.ProcurementMethodDetails)
	}

	return strings.Join(parts, " — ")
}

// extractBuyerProvince finds the province from the buyer party's address.
func extractBuyerProvince(parties []seaoParty) string {
	for i := range parties {
		party := &parties[i]
		for _, role := range party.Roles {
			if role == "buyer" && party.Address.Region != "" {
				return strings.ToLower(party.Address.Region)
			}
		}
	}
	return "qc" // Default for SEAO
}

// extractBuyerCity finds the city from the buyer party's address.
func extractBuyerCity(parties []seaoParty) string {
	for i := range parties {
		party := &parties[i]
		for _, role := range party.Roles {
			if role == "buyer" {
				return party.Address.Locality
			}
		}
	}
	return ""
}

// extractPrimaryUNSPSC returns the first UNSPSC classification ID from tender items.
func extractPrimaryUNSPSC(items []seaoItem) string {
	for _, item := range items {
		if item.Classification.Scheme == classificationSchemeUNSPSC && item.Classification.ID != "" {
			return item.Classification.ID
		}
	}
	return ""
}

// deriveSEAOCategories builds category tags from UNSPSC classifications.
func deriveSEAOCategories(items []seaoItem) []string {
	var categories []string

	for _, item := range items {
		if item.Classification.Scheme != classificationSchemeUNSPSC {
			continue
		}
		id := item.Classification.ID
		if strings.HasPrefix(id, "8111") || strings.HasPrefix(id, "8112") || strings.HasPrefix(id, "8116") {
			categories = append(categories, "it", "it-services")
		} else if strings.HasPrefix(id, "4323") {
			categories = append(categories, "it", "software")
		} else if strings.HasPrefix(id, "43") || strings.HasPrefix(id, "81") {
			categories = append(categories, "it")
		}
	}

	return deduplicate(categories)
}

// hasSEAOITClassification returns true if any tender item has an IT-related UNSPSC code.
func hasSEAOITClassification(items []seaoItem) bool {
	for _, item := range items {
		if item.Classification.Scheme != classificationSchemeUNSPSC {
			continue
		}
		id := item.Classification.ID
		if strings.HasPrefix(id, "43") || strings.HasPrefix(id, "81") {
			return true
		}
	}
	return false
}

// OCDS JSON types for SEAO feeds.

type seaoFeed struct {
	Releases []seaoRelease `json:"releases"`
}

type seaoRelease struct {
	OCID           string      `json:"ocid"`
	ID             string      `json:"id"`
	Date           string      `json:"date"`
	Tag            []string    `json:"tag"`
	InitiationType string      `json:"initiationType"`
	Parties        []seaoParty `json:"parties"`
	Buyer          seaoBuyer   `json:"buyer"`
	Tender         seaoTender  `json:"tender"`
}

type seaoParty struct {
	Name    string      `json:"name"`
	ID      string      `json:"id"`
	Address seaoAddress `json:"address"`
	Roles   []string    `json:"roles"`
}

type seaoAddress struct {
	StreetAddress string `json:"streetAddress"`
	Locality      string `json:"locality"`
	Region        string `json:"region"`
	PostalCode    string `json:"postalCode"`
	CountryName   string `json:"countryName"`
}

type seaoBuyer struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type seaoTender struct {
	ID                       string     `json:"id"`
	Title                    string     `json:"title"`
	Status                   string     `json:"status"`
	Items                    []seaoItem `json:"items"`
	ProcurementMethod        string     `json:"procurementMethod"`
	ProcurementMethodDetails string     `json:"procurementMethodDetails"`
	MainProcurementCategory  string     `json:"mainProcurementCategory"`
	TenderPeriod             seaoPeriod `json:"tenderPeriod"`
}

type seaoItem struct {
	ID             string             `json:"id"`
	Description    string             `json:"description"`
	Classification seaoClassification `json:"classification"`
}

type seaoClassification struct {
	Scheme      string `json:"scheme"`
	ID          string `json:"id"`
	Description string `json:"description"`
}

type seaoPeriod struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}
