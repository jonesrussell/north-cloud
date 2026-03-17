"""Entertainment classifier module — rule-based keyword matching.

Ported from ml-sidecars/entertainment-ml/classifier/relevance.py.
"""

import re

from nc_ml.module import ClassifierModule
from nc_ml.schemas import ClassifierResult, ClassifyRequest

# Relevance classes aligned with original sidecar
CORE_ENTERTAINMENT = "core_entertainment"
PERIPHERAL_ENTERTAINMENT = "peripheral_entertainment"
NOT_ENTERTAINMENT = "not_entertainment"

# Map relevance classes to numeric scores
RELEVANCE_SCORES: dict[str, float] = {
    CORE_ENTERTAINMENT: 0.9,
    PERIPHERAL_ENTERTAINMENT: 0.6,
    NOT_ENTERTAINMENT: 0.1,
}

# Maximum body characters to consider
MAX_BODY_CHARS = 500

# Maximum categories returned
MAX_CATEGORIES = 5

# Strong entertainment signals (reviews, premieres, awards, film/tv/music/games/war films)
CORE_PATTERNS = [
    re.compile(r"\b(film|movie|cinema|box office)\b", re.I),
    re.compile(r"\b(tv show|series|premiere|finale|episode)\b", re.I),
    re.compile(r"\b(album|single|tour|concert|grammy|billboard)\b", re.I),
    re.compile(r"\b(video game|gaming|esports|release date)\b", re.I),
    re.compile(r"\b(review|rating|oscar|emmy|golden globe)\b", re.I),
    re.compile(r"\b(celebrity|starring|cast|trailer)\b", re.I),
    # War-film specific phrases (aimed at film/TV contexts, not general war news)
    re.compile(
        r"\b(war film|war movie|combat film|military drama|world war i film|world war ii film|wwi film|wwii film|vietnam war film|vietnam war movie)\b",
        re.I,
    ),
]

# Weaker signals
PERIPHERAL_PATTERNS = [
    re.compile(r"\b(entertainment|arts|culture)\b", re.I),
    re.compile(r"\b(music|film|television)\b", re.I),
    re.compile(r"\b(streaming|netflix|spotify)\b", re.I),
]

# Keyword -> category for building categories list
CATEGORY_KEYWORDS: list[tuple[list[str], str]] = [
    (["film", "movie", "cinema", "box office", "oscar"], "film"),
    # War-film category (used to generate entertainment:category:war_film channel)
    (
        [
            "war film",
            "war movie",
            "combat film",
            "military drama",
            "wwi film",
            "wwii film",
            "world war i film",
            "world war ii film",
            "vietnam war film",
            "vietnam war movie",
        ],
        "war_film",
    ),
    (["tv", "series", "premiere", "episode", "emmy"], "television"),
    (["album", "song", "concert", "band", "grammy", "music"], "music"),
    (["game", "gaming", "esports"], "gaming"),
    (["review", "rating"], "reviews"),
    (["celebrity", "starring", "cast"], "celebrity"),
]

# Confidence constants
CONFIDENCE_EMPTY = 0.5
CONFIDENCE_CORE_BASE = 0.6
CONFIDENCE_CORE_PER_HIT = 0.1
CONFIDENCE_CORE_MAX = 0.95
CONFIDENCE_PERIPHERAL = 0.65
CONFIDENCE_NOT = 0.6


class EntertainmentResult(ClassifierResult):
    """Entertainment classification result with categories."""

    categories: list[str]


def _extract_categories(text: str) -> list[str]:
    """Extract entertainment categories from text using keyword matching."""
    lower = text.lower()
    categories: list[str] = []
    for keywords, cat in CATEGORY_KEYWORDS:
        if any(kw in lower for kw in keywords) and cat not in categories:
            categories.append(cat)
    return categories[:MAX_CATEGORIES]


class Module(ClassifierModule):
    """Rule-based entertainment classifier module."""

    def name(self) -> str:
        return "entertainment"

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

    async def classify(self, request: ClassifyRequest) -> EntertainmentResult:
        """Classify content for entertainment relevance using keyword rules."""
        body = (request.body or "")[:MAX_BODY_CHARS]
        text = f"{request.title} {body}".strip()

        if not text:
            return EntertainmentResult(
                relevance=RELEVANCE_SCORES[NOT_ENTERTAINMENT],
                confidence=CONFIDENCE_EMPTY,
                categories=[],
            )

        core_hits = sum(1 for p in CORE_PATTERNS if p.search(text))
        peripheral_hits = sum(1 for p in PERIPHERAL_PATTERNS if p.search(text))
        categories = _extract_categories(text)

        if core_hits >= 1:
            confidence = min(CONFIDENCE_CORE_MAX, CONFIDENCE_CORE_BASE + CONFIDENCE_CORE_PER_HIT * core_hits)
            return EntertainmentResult(
                relevance=RELEVANCE_SCORES[CORE_ENTERTAINMENT],
                confidence=round(confidence, 2),
                categories=categories,
            )
        if peripheral_hits >= 1:
            return EntertainmentResult(
                relevance=RELEVANCE_SCORES[PERIPHERAL_ENTERTAINMENT],
                confidence=CONFIDENCE_PERIPHERAL,
                categories=categories,
            )
        return EntertainmentResult(
            relevance=RELEVANCE_SCORES[NOT_ENTERTAINMENT],
            confidence=CONFIDENCE_NOT,
            categories=[],
        )


def create_module() -> Module:
    """Factory function required by the nc_ml framework."""
    return Module()
