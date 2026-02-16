"""Anishinaabe relevance classification (rule-based)."""

import re
from typing import TypedDict

# Relevance classes aligned with plan
CORE_ANISHINAABE = "core_anishinaabe"
PERIPHERAL_ANISHINAABE = "peripheral_anishinaabe"
NOT_ANISHINAABE = "not_anishinaabe"

# Strong Anishinaabe/Indigenous signals
CORE_PATTERNS = [
    re.compile(r"\b(anishinaabe|anishinaabemowin|ojibwe|ojibwa|chippewa)\b", re.I),
    re.compile(r"\b(first nations|indigenous peoples|indigenous community)\b", re.I),
    re.compile(r"\b(métis|metis nation)\b", re.I),
    re.compile(r"\b(inuit|inuk)\b", re.I),
    re.compile(r"\b(residential school|treaty rights|land rights|aboriginal)\b", re.I),
    re.compile(r"\b(seven grandfathers|midewiwin|grand council)\b", re.I),
]

# Weaker signals
PERIPHERAL_PATTERNS = [
    re.compile(r"\b(indigenous|native american|first nation)\b", re.I),
    re.compile(r"\b(reconciliation|truth and reconciliation)\b", re.I),
    re.compile(r"\b(reserve|reservation)\b", re.I),
]

# Keyword -> category for building categories list
CATEGORY_KEYWORDS: list[tuple[list[str], str]] = [
    (["anishinaabe", "ojibwe", "ojibwa", "chippewa", "anishinaabemowin"], "culture"),
    (["language", "anishinaabemowin", "indigenous language"], "language"),
    (["treaty", "governance", "band council", "grand council"], "governance"),
    (["land rights", "territory", "reserve", "reservation"], "land_rights"),
    (["education", "residential school", "reconciliation"], "education"),
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


def classify_anishinaabe_relevance(text: str) -> RelevanceResult:
    """Rule-based classification into core_anishinaabe, peripheral_anishinaabe, or not_anishinaabe."""
    if not text or not text.strip():
        return {
            "relevance": NOT_ANISHINAABE,
            "confidence": 0.5,
            "categories": [],
        }

    core_hits = sum(1 for p in CORE_PATTERNS if p.search(text))
    peripheral_hits = sum(1 for p in PERIPHERAL_PATTERNS if p.search(text))
    categories = _extract_categories(text)

    if core_hits >= 1:
        confidence = min(0.95, 0.6 + 0.1 * core_hits)
        return {
            "relevance": CORE_ANISHINAABE,
            "confidence": round(confidence, 2),
            "categories": categories,
        }
    if peripheral_hits >= 1:
        return {
            "relevance": PERIPHERAL_ANISHINAABE,
            "confidence": 0.65,
            "categories": categories,
        }
    return {
        "relevance": NOT_ANISHINAABE,
        "confidence": 0.6,
        "categories": [],
    }
