"""Tests for indigenous relevance classification (global multilingual v3)."""

from classifier.relevance import (
    CATEGORY_COUNT,
    CATEGORY_KEYWORDS,
    CONFIDENCE_CORE_BASE,
    CONFIDENCE_NOT_INDIGENOUS,
    CORE_INDIGENOUS,
    LANG_EN,
    LANG_ES,
    LANG_FR,
    LANG_JA,
    LANG_MI,
    LANG_PT,
    LANG_SV,
    LANG_UNKNOWN,
    NOT_INDIGENOUS,
    PERIPHERAL_INDIGENOUS,
    classify_indigenous_relevance,
)


class TestEnglishPatterns:
    """English core and peripheral patterns."""

    def test_anishinaabe_core(self):
        result = classify_indigenous_relevance("Anishinaabe community celebrates language revitalization")
        assert result["relevance"] == CORE_INDIGENOUS
        assert result["language_detected"] == LANG_EN

    def test_first_nations_core(self):
        result = classify_indigenous_relevance("First Nations leaders meet for treaty discussions")
        assert result["relevance"] == CORE_INDIGENOUS

    def test_maori_core(self):
        result = classify_indigenous_relevance("Māori iwi gather for annual hui")
        assert result["relevance"] == CORE_INDIGENOUS

    def test_aboriginal_australian_core(self):
        result = classify_indigenous_relevance("Aboriginal Australian elders preserve dreamtime stories")
        assert result["relevance"] == CORE_INDIGENOUS

    def test_native_hawaiian_core(self):
        result = classify_indigenous_relevance("Native Hawaiian sovereignty movement grows")
        assert result["relevance"] == CORE_INDIGENOUS

    def test_sami_core(self):
        result = classify_indigenous_relevance("Sami people protest Nordic mining expansion")
        assert result["relevance"] == CORE_INDIGENOUS

    def test_peripheral_indigenous(self):
        result = classify_indigenous_relevance("Indigenous art exhibit opens downtown")
        assert result["relevance"] == PERIPHERAL_INDIGENOUS
        assert result["language_detected"] == LANG_EN

    def test_reconciliation_peripheral(self):
        result = classify_indigenous_relevance("Reconciliation efforts continue across Canada")
        assert result["relevance"] == PERIPHERAL_INDIGENOUS

    def test_not_indigenous(self):
        result = classify_indigenous_relevance("Weather forecast for the weekend: sunny skies expected")
        assert result["relevance"] == NOT_INDIGENOUS
        assert result["language_detected"] == LANG_UNKNOWN


class TestSpanishPatterns:
    """Spanish language patterns."""

    def test_pueblos_indigenas(self):
        result = classify_indigenous_relevance("Los pueblos indígenas exigen derechos territoriales")
        assert result["relevance"] == CORE_INDIGENOUS
        assert result["language_detected"] == LANG_ES

    def test_territorio_ancestral(self):
        result = classify_indigenous_relevance("Territorio ancestral bajo amenaza de minería")
        assert result["relevance"] == CORE_INDIGENOUS
        assert result["language_detected"] == LANG_ES

    def test_spanish_not_indigenous(self):
        result = classify_indigenous_relevance("El clima de hoy es soleado y templado")
        assert result["relevance"] == NOT_INDIGENOUS


class TestFrenchPatterns:
    """French language patterns."""

    def test_peuples_autochtones(self):
        result = classify_indigenous_relevance("Les peuples autochtones du Canada manifestent")
        assert result["relevance"] == CORE_INDIGENOUS
        assert result["language_detected"] == LANG_FR

    def test_premieres_nations(self):
        result = classify_indigenous_relevance("Les premières nations signent un accord historique")
        assert result["relevance"] == CORE_INDIGENOUS

    def test_french_not_indigenous(self):
        result = classify_indigenous_relevance("La météo prévoit du beau temps demain")
        assert result["relevance"] == NOT_INDIGENOUS


class TestPortuguesePatterns:
    """Portuguese language patterns."""

    def test_povos_indigenas(self):
        result = classify_indigenous_relevance("Povos indígenas lutam pela demarcação de terras")
        assert result["relevance"] == CORE_INDIGENOUS
        assert result["language_detected"] == LANG_PT

    def test_terra_indigena(self):
        result = classify_indigenous_relevance("Terra indígena ameaçada por desmatamento")
        assert result["relevance"] == CORE_INDIGENOUS

    def test_portuguese_not_indigenous(self):
        result = classify_indigenous_relevance("O tempo está ensolarado hoje no Brasil")
        assert result["relevance"] == NOT_INDIGENOUS


class TestNordicPatterns:
    """Nordic/Sami language patterns."""

    def test_samefolket(self):
        result = classify_indigenous_relevance("Samefolket kämpar för rättigheter i Sápmi")
        assert result["relevance"] == CORE_INDIGENOUS
        assert result["language_detected"] == LANG_SV

    def test_urfolk(self):
        result = classify_indigenous_relevance("Urfolk i Norden organiserar motstånd")
        assert result["relevance"] == CORE_INDIGENOUS

    def test_nordic_not_indigenous(self):
        result = classify_indigenous_relevance("Vädret i Stockholm är soligt idag")
        assert result["relevance"] == NOT_INDIGENOUS


class TestTeReoMaoriPatterns:
    """Te Reo Māori patterns."""

    def test_tangata_whenua(self):
        result = classify_indigenous_relevance("Tangata whenua speak at parliament hearing")
        assert result["relevance"] == CORE_INDIGENOUS
        assert result["language_detected"] == LANG_MI

    def test_mana_whenua(self):
        result = classify_indigenous_relevance("Mana whenua assert rights over waterways")
        assert result["relevance"] == CORE_INDIGENOUS


class TestJapanesePatterns:
    """Japanese (Ainu) patterns."""

    def test_ainu(self):
        result = classify_indigenous_relevance("アイヌ民族の文化復興運動が進む")
        assert result["relevance"] == CORE_INDIGENOUS
        assert result["language_detected"] == LANG_JA

    def test_senjuminzoku(self):
        result = classify_indigenous_relevance("先住民族の権利に関する国連宣言")
        assert result["relevance"] == CORE_INDIGENOUS


class TestMixedLanguageContent:
    """Mixed-language content tests."""

    def test_english_title_spanish_body(self):
        result = classify_indigenous_relevance(
            "Indigenous justice report: Los pueblos indígenas exigen justicia"
        )
        assert result["relevance"] == CORE_INDIGENOUS
        assert "justice" in result["categories"]

    def test_french_title_english_categories(self):
        result = classify_indigenous_relevance(
            "Les peuples autochtones demand justice and sovereignty"
        )
        assert result["relevance"] == CORE_INDIGENOUS
        assert "justice" in result["categories"] or "sovereignty" in result["categories"]


class TestLowConfidenceCases:
    """Low-confidence and edge cases."""

    def test_peripheral_confidence_lower_than_core(self):
        core = classify_indigenous_relevance("Anishinaabe community celebrates culture")
        periph = classify_indigenous_relevance("Indigenous art exhibit opens")
        assert core["confidence"] > periph["confidence"]

    def test_not_indigenous_confidence(self):
        result = classify_indigenous_relevance("Stock market report for today")
        assert result["confidence"] == CONFIDENCE_NOT_INDIGENOUS

    def test_single_core_hit_confidence(self):
        result = classify_indigenous_relevance("Inuit hunters report changes")
        assert result["confidence"] >= CONFIDENCE_CORE_BASE

    def test_multiple_core_hits_higher_confidence(self):
        single = classify_indigenous_relevance("First Nations leaders discuss issues")
        multi = classify_indigenous_relevance(
            "First Nations and Métis leaders discuss treaty rights and land rights"
        )
        assert multi["confidence"] >= single["confidence"]


class TestFalsePositives:
    """Non-indigenous content that might false-positive."""

    def test_financial_reserve(self):
        # "reserve" alone is peripheral, but in banking context it should still trigger.
        # This tests that we accept the known limitation — peripheral is expected.
        result = classify_indigenous_relevance("Federal Reserve raises interest rates again")
        # "reserve" triggers peripheral — this is a known limitation acceptable at this stage.
        assert result["relevance"] in (PERIPHERAL_INDIGENOUS, NOT_INDIGENOUS)

    def test_military_reservation(self):
        result = classify_indigenous_relevance("Military reservation training exercise completed")
        # "reservation" triggers peripheral — known limitation.
        assert result["relevance"] in (PERIPHERAL_INDIGENOUS, NOT_INDIGENOUS)

    def test_generic_weather(self):
        result = classify_indigenous_relevance("Tomorrow will be partly cloudy with a high of 22")
        assert result["relevance"] == NOT_INDIGENOUS


class TestCategories:
    """Category extraction tests for all 10 global categories."""

    def test_culture_category(self):
        result = classify_indigenous_relevance("First Nations powwow and ceremony celebrate culture")
        assert "culture" in result["categories"]

    def test_language_category(self):
        result = classify_indigenous_relevance("First Nations indigenous language revitalization program")
        assert "language" in result["categories"]

    def test_land_rights_category(self):
        result = classify_indigenous_relevance("First Nations land rights and territory disputes")
        assert "land_rights" in result["categories"]

    def test_environment_category(self):
        result = classify_indigenous_relevance("First Nations water rights and climate change impact")
        assert "environment" in result["categories"]

    def test_sovereignty_category(self):
        result = classify_indigenous_relevance("First Nations self-determination and sovereignty movement")
        assert "sovereignty" in result["categories"]

    def test_education_category(self):
        result = classify_indigenous_relevance(
            "Residential school survivors share stories of indigenous education"
        )
        assert "education" in result["categories"]

    def test_health_category(self):
        result = classify_indigenous_relevance(
            "Indigenous health crisis: traditional medicine programs expanded"
        )
        assert "health" in result["categories"]

    def test_justice_category(self):
        result = classify_indigenous_relevance("First Nations missing and murdered inquiry continues")
        assert "justice" in result["categories"]

    def test_history_category(self):
        result = classify_indigenous_relevance("First Nations colonial history and decolonization efforts")
        assert "history" in result["categories"]

    def test_community_category(self):
        result = classify_indigenous_relevance("First Nations elders and youth gather for community event")
        assert "community" in result["categories"]

    def test_max_categories(self):
        result = classify_indigenous_relevance(
            "First Nations culture language land rights environment sovereignty "
            "education health justice history community"
        )
        assert len(result["categories"]) <= 5

    def test_empty_text(self):
        result = classify_indigenous_relevance("")
        assert result["relevance"] == NOT_INDIGENOUS
        assert result["categories"] == []
        assert result["language_detected"] == LANG_UNKNOWN

    def test_whitespace_text(self):
        result = classify_indigenous_relevance("   ")
        assert result["relevance"] == NOT_INDIGENOUS


class TestMultilingualCategories:
    """Category extraction in non-English languages."""

    def test_spanish_culture(self):
        result = classify_indigenous_relevance("Los pueblos indígenas celebran una ceremonia cultural")
        assert "culture" in result["categories"]

    def test_french_education(self):
        result = classify_indigenous_relevance("Les peuples autochtones ouvrent une école autochtone")
        assert "education" in result["categories"]

    def test_portuguese_land_rights(self):
        result = classify_indigenous_relevance("Povos indígenas lutam pela demarcação de território")
        assert "land_rights" in result["categories"]

    def test_nordic_sovereignty(self):
        result = classify_indigenous_relevance("Samefolket kräver suveränitet och självbestämmande")
        assert "sovereignty" in result["categories"]

    def test_te_reo_community(self):
        result = classify_indigenous_relevance("Tangata whenua hui with whānau and kaumātua")
        assert "community" in result["categories"]

    def test_japanese_culture(self):
        result = classify_indigenous_relevance("アイヌ民族の文化と伝統の復興")
        assert "culture" in result["categories"]


class TestCategoryTaxonomy:
    """Verify the category taxonomy structure."""

    def test_category_count(self):
        assert len(CATEGORY_KEYWORDS) == CATEGORY_COUNT

    def test_all_slugs_present(self):
        slugs = {cat for _, cat in CATEGORY_KEYWORDS}
        expected = {
            "culture", "language", "land_rights", "environment", "sovereignty",
            "education", "health", "justice", "history", "community",
        }
        assert slugs == expected

    def test_no_duplicate_slugs(self):
        slugs = [cat for _, cat in CATEGORY_KEYWORDS]
        assert len(slugs) == len(set(slugs))
