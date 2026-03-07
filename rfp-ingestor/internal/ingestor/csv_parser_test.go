package ingestor

import (
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/domain"
)

// fullHeader is the complete 67-column header from the CanadaBuys CSV feed.
const fullHeader = `"title-titre-eng","title-titre-fra","referenceNumber-numeroReference","amendmentNumber-numeroModification","solicitationNumber-numeroSollicitation","publicationDate-datePublication","tenderClosingDate-appelOffresDateCloture","amendmentDate-dateModification","expectedContractStartDate-dateDebutContratPrevue","expectedContractEndDate-dateFinContratPrevue","tenderStatus-appelOffresStatut-eng","tenderStatus-appelOffresStatut-fra","gsin-nibs","gsinDescription-nibsDescription-eng","gsinDescription-nibsDescription-fra","unspsc","unspscDescription-eng","unspscDescription-fra","procurementCategory-categorieApprovisionnement","noticeType-avisType-eng","noticeType-avisType-fra","procurementMethod-methodeApprovisionnement-eng","procurementMethod-methodeApprovisionnement-fra","selectionCriteria-criteresSelection-eng","selectionCriteria-criteresSelection-fra","limitedTenderingReason-raisonAppelOffresLimite-eng","limitedTenderingReason-raisonAppelOffresLimite-fra","tradeAgreements-accordsCommerciaux-eng","tradeAgreements-accordsCommerciaux-fra","regionsOfOpportunity-regionAppelOffres-eng","regionsOfOpportunity-regionAppelOffres-fra","regionsOfDelivery-regionsLivraison-eng","regionsOfDelivery-regionsLivraison-fra","contractingEntityName-nomEntitContractante-eng","contractingEntityAddressLine-ligneAdresseEntiteContractante-eng","contractingEntityAddressCity-entiteContractanteAdresseVille-eng","contractingEntityAddressProvince-entiteContractanteAdresseProvince-eng","contractingEntityAddressPostalCode-entiteContractanteAdresseCodePostal","contractingEntityAddressCountry-entiteContractanteAdressePays-eng","contractingEntityName-nomEntitContractante-fra","contractingEntityAddressLine-ligneAdresseEntiteContractante-fra","contractingEntityAddressCity-entiteContractanteAdresseVille-fra","contractingEntityAddressProvince-entiteContractanteAdresseProvince-fra","contractingEntityAddressCountry-entiteContractanteAdressePays-fra","endUserEntitiesName-nomEntitesUtilisateurFinal-eng","endUserEntitiesAddress-adresseEntitesUtilisateurFinal-eng","endUserEntitiesName-nomEntitesUtilisateurFinal-fra","endUserEntitiesAddress-adresseEntitesUtilisateurFinal-fra","contactInfoName-informationsContactNom","contactInfoEmail-informationsContactCourriel","contactInfoPhone-contactInfoTelephone","contactInfoFax","contactInfoAddressLine-contactInfoAdresseLigne-eng","contactInfoCity-contacterInfoVille-eng","contactInfoProvince-contacterInfoProvince-eng","contactInfoPostalcode","contactInfoCountry-contactInfoPays-eng","contactInfoAddressLine-contactInfoAdresseLigne-fra","contactInfoCity-contacterInfoVille-fra","contactInfoProvince-contacterInfoProvince-fra","contactInfoCountry-contactInfoPays-fra","noticeURL-URLavis-eng","noticeURL-URLavis-fra","attachment-piecesJointes-eng","attachment-piecesJointes-fra","tenderDescription-descriptionAppelOffres-eng","tenderDescription-descriptionAppelOffres-fra"`

// makeRow builds a 67-column CSV row with the given field overrides.
// Fields not specified default to empty strings.
func makeRow(fields map[int]string) string {
	cols := make([]string, 67)
	for i := range cols {
		cols[i] = `""`
	}
	for idx, val := range fields {
		cols[idx] = `"` + val + `"`
	}
	return strings.Join(cols, ",")
}

// Column indices for the fields used in test rows (0-based, matching fullHeader).
const (
	idxTitle              = 0
	idxRefNumber          = 2
	idxAmendment          = 3
	idxSolicitationNumber = 4
	idxPubDate            = 5
	idxClosingDate        = 6
	idxAmendmentDate      = 7
	idxStatusEng          = 10
	idxGSIN               = 12
	idxUNSPSC             = 15
	idxProcurementCat     = 18
	idxRegionDelivery     = 31
	idxOrgName            = 33
	idxCity               = 35
	idxContactName        = 48
	idxContactEmail       = 49
	idxNoticeURL          = 61
	idxDescriptionEng     = 65
)

func TestParseCSV_SingleRow(t *testing.T) {
	row := makeRow(map[int]string{
		idxTitle:          "IT Software Modernization Services",
		idxRefNumber:      "PW-24-01234567",
		idxAmendment:      "000",
		idxPubDate:        "2024-11-15",
		idxClosingDate:    "2025-01-15",
		idxAmendmentDate:  "2024-12-01",
		idxStatusEng:      "Open",
		idxGSIN:           "*D121",
		idxUNSPSC:         "*43232300",
		idxProcurementCat: "SV",
		idxRegionDelivery: "National Capital Region (NCR)",
		idxOrgName:        "Shared Services Canada",
		idxCity:           "Ottawa",
		idxContactName:    "Jane Doe",
		idxContactEmail:   "jane.doe@canada.ca",
		idxNoticeURL:      "https://canadabuys.canada.ca/en/tender/PW-24-01234567",
		idxDescriptionEng: "Modernization of legacy IT systems for the Government of Canada.",
	})

	input := fullHeader + "\n" + row + "\n"
	docs, errs := ParseCSV(strings.NewReader(input))

	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}

	doc := docs[0]

	// Top-level fields
	assertEqual(t, "Title", "IT Software Modernization Services", doc.Title)
	assertEqual(t, "URL", "https://canadabuys.canada.ca/en/tender/PW-24-01234567", doc.URL)
	assertEqual(t, "SourceName", "CanadaBuys", doc.SourceName)
	assertEqual(t, "ContentType", "rfp", doc.ContentType)
	if doc.QualityScore != 80 {
		t.Errorf("QualityScore: expected 80, got %d", doc.QualityScore)
	}
	assertEqual(t, "Snippet", "Modernization of legacy IT systems for the Government of Canada.", doc.Snippet)

	if len(doc.Topics) < 1 || doc.Topics[0] != "politics" {
		t.Errorf("Topics: expected first topic 'politics', got %v", doc.Topics)
	}
	if len(doc.Topics) < 2 || doc.Topics[1] != "technology" {
		t.Errorf("Topics: expected second topic 'technology', got %v", doc.Topics)
	}

	// RFP fields
	rfp := doc.RFP
	assertEqual(t, "extraction_method", "csv_feed", rfp.ExtractionMethod)
	assertEqual(t, "reference_number", "PW-24-01234567", rfp.ReferenceNumber)
	assertEqual(t, "organization_name", "Shared Services Canada", rfp.OrganizationName)
	assertEqual(t, "closing_date", "2025-01-15", rfp.ClosingDate)
	assertEqual(t, "province", "on", rfp.Province)
	assertEqual(t, "city", "Ottawa", rfp.City)
	assertEqual(t, "country", "CA", rfp.Country)
	assertEqual(t, "contact_name", "Jane Doe", rfp.ContactName)
	assertEqual(t, "contact_email", "jane.doe@canada.ca", rfp.ContactEmail)
	assertEqual(t, "gsin", "*D121", rfp.GSIN)
	assertEqual(t, "tender_status", "Open", rfp.TenderStatus)
	assertEqual(t, "procurement_type", "services", rfp.ProcurementType)
	assertEqual(t, "budget_currency", "CAD", rfp.BudgetCurrency)

	// Categories should include "it" and "software"
	catSet := toSet(rfp.Categories)
	if _, ok := catSet["it"]; !ok {
		t.Errorf("Categories: expected 'it', got %v", rfp.Categories)
	}
	if _, ok := catSet["software"]; !ok {
		t.Errorf("Categories: expected 'software', got %v", rfp.Categories)
	}
}

func TestParseCSV_SkipsClosedTenders(t *testing.T) {
	row := makeRow(map[int]string{
		idxTitle:     "Closed Tender",
		idxRefNumber: "PW-24-99999999",
		idxAmendment: "000",
		idxStatusEng: "Closed",
	})

	input := fullHeader + "\n" + row + "\n"
	docs, errs := ParseCSV(strings.NewReader(input))

	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 documents for closed tender, got %d", len(docs))
	}
}

func TestParseCSV_EmptyInput(t *testing.T) {
	input := fullHeader + "\n"
	docs, errs := ParseCSV(strings.NewReader(input))

	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 documents for header-only input, got %d", len(docs))
	}
}

func TestParseCSV_SolicitationNumberFallback(t *testing.T) {
	row := makeRow(map[int]string{
		idxTitle:              "Solicitation Only Tender",
		idxSolicitationNumber: "SOL-2024-00999",
		idxAmendment:          "000",
		idxStatusEng:          "Open",
	})

	input := fullHeader + "\n" + row + "\n"
	docs, errs := ParseCSV(strings.NewReader(input))

	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}

	assertEqual(t, "reference_number", "SOL-2024-00999", docs[0].RFP.ReferenceNumber)
}

func TestDocumentID_Deterministic(t *testing.T) {
	doc := domain.RFPDocument{
		RFP: domain.RFP{
			ReferenceNumber: "PW-24-01234567",
			AmendmentNumber: "000",
		},
	}

	id1 := DocumentID(doc)
	id2 := DocumentID(doc)

	if id1 != id2 {
		t.Errorf("DocumentID not deterministic: %q != %q", id1, id2)
	}
	if len(id1) != 64 {
		t.Errorf("expected 64-char hex SHA-256, got %d chars: %q", len(id1), id1)
	}
}

func TestDocumentID_DifferentAmendments(t *testing.T) {
	doc1 := domain.RFPDocument{
		RFP: domain.RFP{
			ReferenceNumber: "PW-24-01234567",
			AmendmentNumber: "000",
		},
	}
	doc2 := domain.RFPDocument{
		RFP: domain.RFP{
			ReferenceNumber: "PW-24-01234567",
			AmendmentNumber: "001",
		},
	}

	id1 := DocumentID(doc1)
	id2 := DocumentID(doc2)

	if id1 == id2 {
		t.Errorf("expected different IDs for different amendments, both got %q", id1)
	}
}

func TestNormalizeProvince(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Ontario", "on"},
		{"*Ontario", "on"},
		{"National Capital Region (NCR)", "on"},
		{"British Columbia", "bc"},
		{"Canada", ""},
		{"", ""},
		{"*British Columbia", "bc"},
		{"ONTARIO", "on"},
		{"  Ontario  ", "on"},
		{"Alberta", "ab"},
		{"Quebec", "qc"},
		{"Saskatchewan", "sk"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeProvince(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeProvince(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDeriveCategories(t *testing.T) {
	tests := []struct {
		name       string
		gsin       string
		unspsc     string
		wantSubset []string
	}{
		{
			name:       "GSIN *D prefix indicates IT",
			gsin:       "*D121",
			unspsc:     "",
			wantSubset: []string{"it"},
		},
		{
			name:       "UNSPSC *4323 indicates software",
			gsin:       "",
			unspsc:     "*43232300",
			wantSubset: []string{"it", "software"},
		},
		{
			name:       "UNSPSC *8111 indicates IT services",
			gsin:       "",
			unspsc:     "*81112200",
			wantSubset: []string{"it", "it-services"},
		},
		{
			name:       "UNSPSC *8112 indicates IT services",
			gsin:       "",
			unspsc:     "*81121500",
			wantSubset: []string{"it", "it-services"},
		},
		{
			name:       "UNSPSC *43 broad IT",
			gsin:       "",
			unspsc:     "*43000000",
			wantSubset: []string{"it"},
		},
		{
			name:       "non-IT GSIN and UNSPSC",
			gsin:       "*N100",
			unspsc:     "*12345678",
			wantSubset: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveCategories(tt.gsin, tt.unspsc)
			gotSet := toSet(got)

			for _, want := range tt.wantSubset {
				if _, ok := gotSet[want]; !ok {
					t.Errorf("deriveCategories(%q, %q) = %v, missing %q", tt.gsin, tt.unspsc, got, want)
				}
			}

			if tt.wantSubset == nil && len(got) != 0 {
				t.Errorf("deriveCategories(%q, %q) = %v, want empty", tt.gsin, tt.unspsc, got)
			}
		})
	}
}

func TestParseCSV_ConstructsURLWhenMissing(t *testing.T) {
	row := makeRow(map[int]string{
		idxTitle:     "Missing URL Tender",
		idxRefNumber: "cb-651-86492266",
		idxAmendment: "000",
		idxStatusEng: "Open",
		// idxNoticeURL intentionally omitted (empty)
	})

	input := fullHeader + "\n" + row + "\n"
	docs, errs := ParseCSV(strings.NewReader(input))

	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}

	wantURL := "https://canadabuys.canada.ca/en/tender-opportunities/tender-notice/cb-651-86492266"
	assertEqual(t, "URL", wantURL, docs[0].URL)
	assertEqual(t, "RFP.SourceURL", wantURL, docs[0].RFP.SourceURL)
}

func TestParseCSV_PreservesExplicitURL(t *testing.T) {
	provided := "https://canadabuys.canada.ca/en/tender/PW-24-01234567"
	row := makeRow(map[int]string{
		idxTitle:     "Explicit URL Tender",
		idxRefNumber: "PW-24-01234567",
		idxAmendment: "000",
		idxStatusEng: "Open",
		idxNoticeURL: provided,
	})

	input := fullHeader + "\n" + row + "\n"
	docs, errs := ParseCSV(strings.NewReader(input))

	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}

	assertEqual(t, "URL", provided, docs[0].URL)
}

func TestParseCSV_BOMHeader(t *testing.T) {
	// CanadaBuys feeds start with a UTF-8 BOM (\xEF\xBB\xBF).
	// Verify the parser strips it so the first column is still matched.
	row := makeRow(map[int]string{
		idxTitle:     "BOM Test Tender",
		idxRefNumber: "PW-BOM-001",
		idxAmendment: "000",
		idxStatusEng: "Open",
	})

	input := "\xEF\xBB\xBF" + fullHeader + "\n" + row + "\n"
	docs, errs := ParseCSV(strings.NewReader(input))

	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	assertEqual(t, "Title", "BOM Test Tender", docs[0].Title)
}

// assertEqual is a test helper that checks string equality.
func assertEqual(t *testing.T, field, expected, actual string) {
	t.Helper()
	if expected != actual {
		t.Errorf("%s: expected %q, got %q", field, expected, actual)
	}
}

// toSet converts a string slice to a set for membership testing.
func toSet(items []string) map[string]struct{} {
	s := make(map[string]struct{}, len(items))
	for _, item := range items {
		s[item] = struct{}{}
	}
	return s
}
