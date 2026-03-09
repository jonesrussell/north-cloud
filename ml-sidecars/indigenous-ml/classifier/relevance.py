"""Indigenous relevance classification (rule-based, global multilingual)."""

import re
from typing import TypedDict

# Relevance classes
CORE_INDIGENOUS = "core_indigenous"
PERIPHERAL_INDIGENOUS = "peripheral_indigenous"
NOT_INDIGENOUS = "not_indigenous"

# Maximum number of categories returned per classification.
MAX_CATEGORIES = 5

# --- Strong Indigenous signals (multilingual) ---
CORE_PATTERNS = [
    # English (expanded global)
    re.compile(r"\b(anishinaabe|anishinaabemowin|ojibwe|ojibwa|chippewa)\b", re.I),
    re.compile(r"\b(first nations|indigenous peoples|indigenous community)\b", re.I),
    re.compile(r"\b(m[eé]tis|metis nation)\b", re.I),
    re.compile(r"\b(inuit|inuk)\b", re.I),
    re.compile(r"\b(residential school|treaty rights|land rights|aboriginal)\b", re.I),
    re.compile(r"\b(seven grandfathers|midewiwin|grand council)\b", re.I),
    re.compile(r"\b(m[aā]ori|iwi|hap[uū]|wh[aā]nau)\b", re.I),
    re.compile(r"\b(native hawaiian|tribal sovereignty|tribal nation)\b", re.I),
    re.compile(r"\b(aboriginal australian|torres strait islander)\b", re.I),
    re.compile(r"\b(sami people|sámi|saami)\b", re.I),
    # Spanish
    re.compile(r"\b(pueblos ind[ií]genas|comunidad ind[ií]gena)\b", re.I),
    re.compile(r"\b(territorio ancestral|derechos ind[ií]genas)\b", re.I),
    # French
    re.compile(r"\b(peuples autochtones|premi[eè]res nations)\b", re.I),
    re.compile(r"\b(droits autochtones|communaut[eé] autochtone)\b", re.I),
    # Portuguese
    re.compile(r"\b(povos ind[ií]genas|terra ind[ií]gena|demarca[cç][aã]o)\b", re.I),
    # Nordic (Sami)
    re.compile(r"\b(samefolket|urfolk|samisk|s[aá]pmi)\b", re.I),
    re.compile(r"\b(alkuper[aä]iskansa|ursprungsfolk)\b", re.I),
    # Te Reo Māori
    re.compile(r"\b(tangata whenua|te tiriti|mana whenua)\b", re.I),
    # Japanese (Ainu)
    re.compile(r"(アイヌ|先住民族|アイヌ民族)"),
]

# --- Weaker signals (multilingual) ---
PERIPHERAL_PATTERNS = [
    re.compile(r"\b(indigenous|native american|first nation)\b", re.I),
    re.compile(r"\b(reconciliation|truth and reconciliation)\b", re.I),
    re.compile(r"\b(reserve|reservation)\b", re.I),
    re.compile(r"\b(autochtone|autochton)\b", re.I),
    re.compile(r"\b(ind[ií]gena)\b", re.I),
]

# --- 10 global categories ---
# Each tuple: (keyword_list, category_slug).
# Keywords are matched as substrings against lowercased text.
# D2.1 will expand each list with domain-expert-reviewed multilingual terms.
CATEGORY_COUNT = 10
CATEGORY_KEYWORDS: list[tuple[list[str], str]] = [
    # Culture: ceremonies, art, music, dance, traditional practices
    (["culture", "ceremony", "powwow", "potlatch", "sweat lodge", "corroboree",
      "haka", "dreamtime",
      "cultura", "ceremonia",  # Spanish
      "cérémonie", "tradition",  # French
      # Portuguese / Nordic / Te Reo / Japanese — D2.1 placeholder
      ], "culture"),
    # Language: revitalization, education, documentation, endangered languages
    (["language", "anishinaabemowin", "indigenous language", "cree", "inuktitut",
      "te reo",
      "lengua indígena",  # Spanish
      "langue autochtone",  # French
      # Portuguese / Nordic / Te Reo / Japanese — D2.1 placeholder
      ], "language"),
    # Land rights: territory disputes, land claims, demarcation
    (["land rights", "territory", "reserve", "reservation", "land claim",
      "territorio ancestral",  # Spanish
      "terra indígena", "demarcação",  # Portuguese
      # French / Nordic / Te Reo / Japanese — D2.1 placeholder
      ], "land_rights"),
    # Environment: climate, water rights, pipeline opposition, conservation
    (["environment", "climate", "water rights", "pipeline", "deforestation",
      "medio ambiente",  # Spanish
      "environnement",  # French
      # Portuguese / Nordic / Te Reo / Japanese — D2.1 placeholder
      ], "environment"),
    # Sovereignty: self-determination, governance, treaties, political autonomy
    (["sovereignty", "self-determination", "self-governance", "treaty",
      "governance", "band council", "grand council",
      "soberanía", "autodeterminación",  # Spanish
      # French / Portuguese / Nordic / Te Reo / Japanese — D2.1 placeholder
      ], "sovereignty"),
    # Education: schools, residential school legacy, indigenous education programs
    (["education", "residential school", "indigenous education",
      "educación",  # Spanish
      "éducation",  # French
      # Portuguese / Nordic / Te Reo / Japanese — D2.1 placeholder
      ], "education"),
    # Health: indigenous health disparities, traditional medicine
    (["health", "indigenous health", "traditional medicine",
      "salud indígena",  # Spanish
      "santé autochtone",  # French
      # Portuguese / Nordic / Te Reo / Japanese — D2.1 placeholder
      ], "health"),
    # Justice: MMIWG, incarceration, policing, legal rights
    (["justice", "missing and murdered", "incarceration", "police",
      "justicia",  # Spanish
      "justice autochtone",  # French
      # Portuguese / Nordic / Te Reo / Japanese — D2.1 placeholder
      ], "justice"),
    # History: colonial history, decolonization, historical events
    (["history", "colonial", "colonization", "decolonization",
      "historia",  # Spanish
      "histoire", "colonisation",  # French
      # Portuguese / Nordic / Te Reo / Japanese — D2.1 placeholder
      ], "history"),
    # Community: elders, youth, family, community events
    (["community", "elders", "youth",
      "whānau", "hapū",  # Te Reo
      "comunidad",  # Spanish
      "communauté",  # French
      # Portuguese / Nordic / Japanese — D2.1 placeholder
      ], "community"),
]


class RelevanceResult(TypedDict):
    relevance: str
    confidence: float
    categories: list[str]


def _extract_categories(text: str) -> list[str]:
    lower = text.lower()
    categories: list[str] = []
    for keywords, cat in CATEGORY_KEYWORDS:
        if any(kw in lower for kw in keywords) and cat not in categories:
            categories.append(cat)
    return categories[:MAX_CATEGORIES]


def classify_indigenous_relevance(text: str) -> RelevanceResult:
    """Rule-based classification into core_indigenous, peripheral_indigenous, or not_indigenous."""
    if not text or not text.strip():
        return {
            "relevance": NOT_INDIGENOUS,
            "confidence": 0.5,
            "categories": [],
        }

    core_hits = sum(1 for p in CORE_PATTERNS if p.search(text))
    peripheral_hits = sum(1 for p in PERIPHERAL_PATTERNS if p.search(text))
    categories = _extract_categories(text)

    if core_hits >= 1:
        confidence = min(0.95, 0.6 + 0.1 * core_hits)
        return {
            "relevance": CORE_INDIGENOUS,
            "confidence": round(confidence, 2),
            "categories": categories,
        }
    if peripheral_hits >= 1:
        return {
            "relevance": PERIPHERAL_INDIGENOUS,
            "confidence": 0.65,
            "categories": categories,
        }
    return {
        "relevance": NOT_INDIGENOUS,
        "confidence": 0.6,
        "categories": [],
    }
