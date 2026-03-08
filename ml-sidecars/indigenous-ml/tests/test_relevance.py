"""Tests for indigenous relevance classification (global multilingual)."""

from classifier.relevance import (
    CORE_INDIGENOUS,
    NOT_INDIGENOUS,
    PERIPHERAL_INDIGENOUS,
    classify_indigenous_relevance,
)


class TestEnglishPatterns:
    """English core and peripheral patterns."""

    def test_anishinaabe_core(self):
        result = classify_indigenous_relevance("Anishinaabe community celebrates language revitalization")
        assert result["relevance"] == CORE_INDIGENOUS

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

    def test_reconciliation_peripheral(self):
        result = classify_indigenous_relevance("Reconciliation efforts continue across Canada")
        assert result["relevance"] == PERIPHERAL_INDIGENOUS

    def test_not_indigenous(self):
        result = classify_indigenous_relevance("Weather forecast for the weekend: sunny skies expected")
        assert result["relevance"] == NOT_INDIGENOUS


class TestSpanishPatterns:
    """Spanish language patterns."""

    def test_pueblos_indigenas(self):
        result = classify_indigenous_relevance("Los pueblos indígenas exigen derechos territoriales")
        assert result["relevance"] == CORE_INDIGENOUS

    def test_territorio_ancestral(self):
        result = classify_indigenous_relevance("Territorio ancestral bajo amenaza de minería")
        assert result["relevance"] == CORE_INDIGENOUS


class TestFrenchPatterns:
    """French language patterns."""

    def test_peuples_autochtones(self):
        result = classify_indigenous_relevance("Les peuples autochtones du Canada manifestent")
        assert result["relevance"] == CORE_INDIGENOUS

    def test_premieres_nations(self):
        result = classify_indigenous_relevance("Les premières nations signent un accord historique")
        assert result["relevance"] == CORE_INDIGENOUS


class TestPortuguesePatterns:
    """Portuguese language patterns."""

    def test_povos_indigenas(self):
        result = classify_indigenous_relevance("Povos indígenas lutam pela demarcação de terras")
        assert result["relevance"] == CORE_INDIGENOUS

    def test_terra_indigena(self):
        result = classify_indigenous_relevance("Terra indígena ameaçada por desmatamento")
        assert result["relevance"] == CORE_INDIGENOUS


class TestNordicPatterns:
    """Nordic/Sami language patterns."""

    def test_samefolket(self):
        result = classify_indigenous_relevance("Samefolket kämpar för rättigheter i Sápmi")
        assert result["relevance"] == CORE_INDIGENOUS

    def test_urfolk(self):
        result = classify_indigenous_relevance("Urfolk i Norden organiserar motstånd")
        assert result["relevance"] == CORE_INDIGENOUS


class TestTeReoMaoriPatterns:
    """Te Reo Māori patterns."""

    def test_tangata_whenua(self):
        result = classify_indigenous_relevance("Tangata whenua speak at parliament hearing")
        assert result["relevance"] == CORE_INDIGENOUS

    def test_mana_whenua(self):
        result = classify_indigenous_relevance("Mana whenua assert rights over waterways")
        assert result["relevance"] == CORE_INDIGENOUS


class TestJapanesePatterns:
    """Japanese (Ainu) patterns."""

    def test_ainu(self):
        result = classify_indigenous_relevance("アイヌ民族の文化復興運動が進む")
        assert result["relevance"] == CORE_INDIGENOUS

    def test_senjuminzoku(self):
        result = classify_indigenous_relevance("先住民族の権利に関する国連宣言")
        assert result["relevance"] == CORE_INDIGENOUS


class TestCategories:
    """Category extraction tests."""

    def test_culture_category(self):
        result = classify_indigenous_relevance("First Nations powwow and ceremony celebrate culture")
        assert "culture" in result["categories"]

    def test_land_rights_category(self):
        result = classify_indigenous_relevance("First Nations land rights and territory disputes")
        assert "land_rights" in result["categories"]

    def test_education_category(self):
        result = classify_indigenous_relevance("Residential school survivors share stories of indigenous education")
        assert "education" in result["categories"]

    def test_health_category(self):
        result = classify_indigenous_relevance("Indigenous health crisis: traditional medicine programs expanded")
        assert "health" in result["categories"]

    def test_max_categories(self):
        result = classify_indigenous_relevance(
            "First Nations culture language land rights environment sovereignty education health justice history community"
        )
        assert len(result["categories"]) <= 5

    def test_empty_text(self):
        result = classify_indigenous_relevance("")
        assert result["relevance"] == NOT_INDIGENOUS
        assert result["categories"] == []

    def test_whitespace_text(self):
        result = classify_indigenous_relevance("   ")
        assert result["relevance"] == NOT_INDIGENOUS
