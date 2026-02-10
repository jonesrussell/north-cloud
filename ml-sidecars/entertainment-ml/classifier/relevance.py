"""Entertainment relevance classification (rule-based)."""

import re
from typing import TypedDict

# Relevance classes aligned with plan
CORE_ENTERTAINMENT = "core_entertainment"
PERIPHERAL_ENTERTAINMENT = "peripheral_entertainment"
NOT_ENTERTAINMENT = "not_entertainment"

# Strong entertainment signals (reviews, premieres, awards, film/tv/music/games)
CORE_PATTERNS = [
    re.compile(r"\b(film|movie|cinema|box office)\b", re.I),
    re.compile(r"\b(tv show|series|premiere|finale|episode)\b", re.I),
    re.compile(r"\b(album|single|tour|concert|grammy|billboard)\b", re.I),
    re.compile(r"\b(video game|gaming|esports|release date)\b", re.I),
    re.compile(r"\b(review|rating|oscar|emmy|golden globe)\b", re.I),
    re.compile(r"\b(celebrity|starring|cast|trailer)\b", re.I),
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
    (["tv", "series", "premiere", "episode", "emmy"], "television"),
    (["album", "song", "concert", "band", "grammy", "music"], "music"),
    (["game", "gaming", "esports"], "gaming"),
    (["review", "rating"], "reviews"),
    (["celebrity", "starring", "cast"], "celebrity"),
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
    return categories[:5]


def classify_entertainment_relevance(text: str) -> RelevanceResult:
    """Rule-based classification into core_entertainment, peripheral_entertainment, or not_entertainment."""
    if not text or not text.strip():
        return {
            "relevance": NOT_ENTERTAINMENT,
            "confidence": 0.5,
            "categories": [],
        }

    core_hits = sum(1 for p in CORE_PATTERNS if p.search(text))
    peripheral_hits = sum(1 for p in PERIPHERAL_PATTERNS if p.search(text))
    categories = _extract_categories(text)

    if core_hits >= 1:
        confidence = min(0.95, 0.6 + 0.1 * core_hits)
        return {
            "relevance": CORE_ENTERTAINMENT,
            "confidence": round(confidence, 2),
            "categories": categories,
        }
    if peripheral_hits >= 1:
        return {
            "relevance": PERIPHERAL_ENTERTAINMENT,
            "confidence": 0.65,
            "categories": categories,
        }
    return {
        "relevance": NOT_ENTERTAINMENT,
        "confidence": 0.6,
        "categories": [],
    }
