"""Indigenous classifier module — rule-based keyword matching (global multilingual v3).

Ported from ml-sidecars/indigenous-ml/classifier/relevance.py.
"""

import re

from nc_ml.module import ClassifierModule
from nc_ml.schemas import ClassifierResult, ClassifyRequest

# Relevance classes
CORE_INDIGENOUS = "core_indigenous"
PERIPHERAL_INDIGENOUS = "peripheral_indigenous"
NOT_INDIGENOUS = "not_indigenous"

# Map relevance classes to numeric scores
RELEVANCE_SCORES: dict[str, float] = {
    CORE_INDIGENOUS: 0.9,
    PERIPHERAL_INDIGENOUS: 0.6,
    NOT_INDIGENOUS: 0.1,
}

# Maximum body characters to consider
MAX_BODY_CHARS = 500

# Maximum number of categories returned per classification.
MAX_CATEGORIES = 5

# Confidence scoring constants.
CONFIDENCE_CORE_BASE = 0.60
CONFIDENCE_CORE_PER_HIT = 0.10
CONFIDENCE_CORE_MAX = 0.95
CONFIDENCE_PERIPHERAL_BASE = 0.55
CONFIDENCE_CATEGORY_BONUS_PER = 0.03
CONFIDENCE_CATEGORY_BONUS_MAX = 0.10
CONFIDENCE_NOT_INDIGENOUS = 0.60
CONFIDENCE_EMPTY = 0.50

# Language codes for detection.
LANG_EN = "en"
LANG_ES = "es"
LANG_FR = "fr"
LANG_PT = "pt"
LANG_SV = "sv"
LANG_MI = "mi"
LANG_JA = "ja"
LANG_UNKNOWN = "unknown"

# --- Strong Indigenous signals (multilingual) ---
# Each tuple: (compiled_regex, language_code)
CORE_PATTERNS: list[tuple[re.Pattern[str], str]] = [
    # English (Canada / North America)
    (re.compile(r"\b(anishinaabe|anishinaabemowin|ojibwe|ojibwa|chippewa)\b", re.I), LANG_EN),
    (re.compile(r"\b(first nations|indigenous peoples|indigenous community)\b", re.I), LANG_EN),
    (re.compile(r"\b(m[eé]tis|metis nation)\b", re.I), LANG_EN),
    (re.compile(r"\b(inuit|inuk)\b", re.I), LANG_EN),
    (re.compile(r"\b(residential school|treaty rights|land rights|aboriginal)\b", re.I), LANG_EN),
    (re.compile(r"\b(seven grandfathers|midewiwin|grand council)\b", re.I), LANG_EN),
    # English (Oceania)
    (re.compile(r"\b(m[aā]ori|iwi|hap[uū]|wh[aā]nau)\b", re.I), LANG_EN),
    (re.compile(r"\b(aboriginal australian|torres strait islander)\b", re.I), LANG_EN),
    # English (US / Hawaii)
    (re.compile(r"\b(native hawaiian|tribal sovereignty|tribal nation)\b", re.I), LANG_EN),
    # English (Nordic)
    (re.compile(r"\b(sami people|sámi|saami)\b", re.I), LANG_EN),
    # Spanish
    (re.compile(r"\b(pueblos ind[ií]genas|comunidad ind[ií]gena)\b", re.I), LANG_ES),
    (re.compile(r"\b(territorio ancestral|derechos ind[ií]genas)\b", re.I), LANG_ES),
    # French
    (re.compile(r"\b(peuples autochtones|premi[eè]res nations)\b", re.I), LANG_FR),
    (re.compile(r"\b(droits autochtones|communaut[eé] autochtone)\b", re.I), LANG_FR),
    # Portuguese
    (re.compile(r"\b(povos ind[ií]genas|terra ind[ií]gena|demarca[cç][aã]o)\b", re.I), LANG_PT),
    # Nordic (Sami)
    (re.compile(r"\b(samefolket|urfolk|samisk|s[aá]pmi)\b", re.I), LANG_SV),
    (re.compile(r"\b(alkuper[aä]iskansa|ursprungsfolk)\b", re.I), LANG_SV),
    # Te Reo Māori
    (re.compile(r"\b(tangata whenua|te tiriti|mana whenua)\b", re.I), LANG_MI),
    # Japanese (Ainu)
    (re.compile(r"(アイヌ|先住民族|アイヌ民族)"), LANG_JA),
]

# --- Weaker signals (multilingual) ---
PERIPHERAL_PATTERNS: list[tuple[re.Pattern[str], str]] = [
    (re.compile(r"\b(indigenous|native american|first nation)\b", re.I), LANG_EN),
    (re.compile(r"\b(reconciliation|truth and reconciliation)\b", re.I), LANG_EN),
    (re.compile(r"\b(reserve|reservation)\b", re.I), LANG_EN),
    (re.compile(r"\b(autochtone?)\b", re.I), LANG_FR),
    (re.compile(r"\b(ind[ií]gena)\b", re.I), LANG_ES),
]

# --- 10 global categories with full multilingual keywords ---
CATEGORY_KEYWORDS: list[tuple[list[str], str]] = [
    # Culture: ceremonies, art, music, dance, traditional practices
    ([
        "culture", "ceremony", "powwow", "potlatch", "sweat lodge", "corroboree",
        "haka", "dreamtime", "totem", "regalia", "storytelling", "sacred",
        "cultura", "ceremonia", "ritual", "tradición",  # Spanish
        "cérémonie", "tradition", "rituel",  # French
        "cerimônia",  # Portuguese
        "kultur", "ceremoni", "sedvänja",  # Nordic
        "tikanga", "whakairo", "kapa haka",  # Te Reo
        "文化", "儀式", "伝統",  # Japanese
    ], "culture"),
    # Language: revitalization, education, documentation, endangered languages
    ([
        "language", "anishinaabemowin", "indigenous language", "cree", "inuktitut",
        "te reo", "immersion", "language revitalization",
        "lengua indígena", "idioma", "revitalización lingüística",  # Spanish
        "langue autochtone", "revitalisation linguistique",  # French
        "língua indígena", "revitalização",  # Portuguese
        "språk", "modersmål", "samiska",  # Nordic
        "reo", "te reo māori", "kōrero",  # Te Reo
        "言語", "アイヌ語", "母語",  # Japanese
    ], "language"),
    # Land rights: territory disputes, land claims, demarcation
    ([
        "land rights", "territory", "reserve", "reservation", "land claim",
        "land back", "native title", "dispossession",
        "territorio ancestral", "derechos territoriales", "tierras indígenas",  # Spanish
        "droits fonciers", "revendication territoriale",  # French
        "terra indígena", "demarcação", "território",  # Portuguese
        "markrättigheter", "renbetesland",  # Nordic
        "whenua", "mana whenua", "raupatu",  # Te Reo
        "土地権利", "領土",  # Japanese
    ], "land_rights"),
    # Environment: climate, water rights, pipeline opposition, conservation
    ([
        "environment", "climate", "water rights", "pipeline", "deforestation",
        "conservation", "sacred site", "ecological",
        "medio ambiente", "deforestación", "recursos naturales",  # Spanish
        "environnement", "changement climatique", "ressources",  # French
        "meio ambiente", "desmatamento", "conservação",  # Portuguese
        "miljö", "klimat", "naturresurser",  # Nordic
        "taiao", "kaitiakitanga", "wai",  # Te Reo
        "環境", "気候", "自然保護",  # Japanese
    ], "environment"),
    # Sovereignty: self-determination, governance, treaties, political autonomy
    ([
        "sovereignty", "self-determination", "self-governance", "treaty",
        "governance", "band council", "grand council", "nation-to-nation",
        "soberanía", "autodeterminación", "autogobierno",  # Spanish
        "souveraineté", "autodétermination", "gouvernance",  # French
        "soberania", "autodeterminação", "governança",  # Portuguese
        "suveränitet", "självbestämmande",  # Nordic
        "tino rangatiratanga", "mana motuhake",  # Te Reo
        "主権", "自決権",  # Japanese
    ], "sovereignty"),
    # Education: schools, residential school legacy, indigenous education programs
    ([
        "education", "residential school", "indigenous education",
        "boarding school", "curriculum", "scholarship",
        "educación", "escuela", "currículo indígena",  # Spanish
        "éducation", "pensionnat", "école autochtone",  # French
        "educação", "escola indígena",  # Portuguese
        "utbildning", "skola", "sameskola",  # Nordic
        "mātauranga", "kura", "wānanga",  # Te Reo
        "教育", "学校",  # Japanese
    ], "education"),
    # Health: indigenous health disparities, traditional medicine
    ([
        "health", "indigenous health", "traditional medicine",
        "mental health", "healing", "wellness",
        "salud indígena", "medicina tradicional",  # Spanish
        "santé autochtone", "médecine traditionnelle",  # French
        "saúde indígena",  # Portuguese
        "hälsa", "traditionell medicin",  # Nordic
        "hauora", "rongoā",  # Te Reo
        "健康", "伝統医療",  # Japanese
    ], "health"),
    # Justice: MMIWG, incarceration, policing, legal rights
    ([
        "justice", "missing and murdered", "incarceration", "police",
        "mmiwg", "inquiry", "legal rights", "discrimination",
        "justicia", "discriminación", "derechos legales",  # Spanish
        "justice autochtone", "enquête", "discrimination",  # French
        "justiça", "discriminação", "direitos",  # Portuguese
        "rättvisa", "diskriminering",  # Nordic
        "ture", "manatika",  # Te Reo
        "正義", "差別",  # Japanese
    ], "justice"),
    # History: colonial history, decolonization, historical events
    ([
        "history", "colonial", "colonization", "decolonization",
        "genocide", "assimilation",
        "historia", "colonización", "descolonización",  # Spanish
        "histoire", "colonisation", "décolonisation",  # French
        "história", "colonização", "descolonização",  # Portuguese
        "historia", "kolonisering",  # Nordic
        "hītori", "whakapapa",  # Te Reo
        "歴史", "植民地",  # Japanese
    ], "history"),
    # Community: elders, youth, family, community events
    ([
        "community", "elders", "youth", "gathering", "assembly", "family",
        "comunidad", "ancianos", "juventud", "asamblea",  # Spanish
        "communauté", "aînés", "jeunesse", "rassemblement",  # French
        "comunidade", "anciãos", "juventude",  # Portuguese
        "gemenskap", "samhälle",  # Nordic
        "whānau", "hapū", "hui", "kaumātua",  # Te Reo
        "コミュニティ", "長老", "集会",  # Japanese
    ], "community"),
]


class IndigenousResult(ClassifierResult):
    """Indigenous classification result with categories."""

    categories: list[str]


def _extract_categories(text: str) -> list[str]:
    """Extract indigenous categories from text using keyword matching."""
    lower = text.lower()
    categories: list[str] = []
    for keywords, cat in CATEGORY_KEYWORDS:
        if any(kw in lower for kw in keywords) and cat not in categories:
            categories.append(cat)
    return categories[:MAX_CATEGORIES]


def _detect_language(text: str) -> str:
    """Return the language code of the first matching core or peripheral pattern."""
    for pattern, lang in CORE_PATTERNS:
        if pattern.search(text):
            return lang
    for pattern, lang in PERIPHERAL_PATTERNS:
        if pattern.search(text):
            return lang
    return LANG_UNKNOWN


class Module(ClassifierModule):
    """Rule-based indigenous classifier module (global multilingual v3)."""

    def name(self) -> str:
        return "indigenous"

    def version(self) -> str:
        return "1.0.0"

    def schema_version(self) -> str:
        return "1.0.0"

    async def initialize(self) -> None:
        """No initialization needed for rule-based module."""

    async def shutdown(self) -> None:
        """No cleanup needed for rule-based module."""

    async def health_checks(self) -> dict[str, bool]:
        """Rule-based module has no model dependencies to check."""
        return {}

    async def classify(self, request: ClassifyRequest) -> IndigenousResult:
        """Classify content for indigenous relevance using keyword rules."""
        body = (request.body or "")[:MAX_BODY_CHARS]
        text = f"{request.title} {body}".strip()

        if not text:
            return IndigenousResult(
                relevance=RELEVANCE_SCORES[NOT_INDIGENOUS],
                confidence=CONFIDENCE_EMPTY,
                categories=[],
            )

        core_hits = sum(1 for p, _ in CORE_PATTERNS if p.search(text))
        peripheral_hits = sum(1 for p, _ in PERIPHERAL_PATTERNS if p.search(text))
        categories = _extract_categories(text)
        category_bonus = min(CONFIDENCE_CATEGORY_BONUS_MAX, len(categories) * CONFIDENCE_CATEGORY_BONUS_PER)

        if core_hits >= 1:
            confidence = min(
                CONFIDENCE_CORE_MAX,
                CONFIDENCE_CORE_BASE + CONFIDENCE_CORE_PER_HIT * core_hits + category_bonus,
            )
            return IndigenousResult(
                relevance=RELEVANCE_SCORES[CORE_INDIGENOUS],
                confidence=round(confidence, 2),
                categories=categories,
            )
        if peripheral_hits >= 1:
            confidence = round(CONFIDENCE_PERIPHERAL_BASE + category_bonus, 2)
            return IndigenousResult(
                relevance=RELEVANCE_SCORES[PERIPHERAL_INDIGENOUS],
                confidence=confidence,
                categories=categories,
            )
        return IndigenousResult(
            relevance=RELEVANCE_SCORES[NOT_INDIGENOUS],
            confidence=CONFIDENCE_NOT_INDIGENOUS,
            categories=[],
        )
