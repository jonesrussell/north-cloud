# coforge-ml/classifier/preprocessor.py
"""Text preprocessing for ML classification."""

import re
from typing import Optional

# Cap input length to bound regex work and avoid ReDoS on adversarial input (CodeQL py/polynomial-redos).
MAX_PREPROCESS_LENGTH = 1_000_000

# ReDoS-safe patterns: avoid ambiguous greedy repetition (e.g. \S+@\S+).
# URL: one [^\s]+ so no backtracking overlap.
_URL_PATTERN = re.compile(r'https?://[^\s]+')
# Email: [^\s@]+ before and after @ so the two parts cannot match the same span.
_EMAIL_PATTERN = re.compile(r'[^\s@]+@[^\s@]+')


def preprocess_text(text: Optional[str]) -> str:
    """Clean and normalize text for vectorization.

    Args:
        text: Raw text input, may be None

    Returns:
        Cleaned, lowercase text with URLs/emails removed
    """
    if not text:
        return ""

    if len(text) > MAX_PREPROCESS_LENGTH:
        text = text[:MAX_PREPROCESS_LENGTH]

    # Lowercase
    text = text.lower()

    # Remove URLs (single [^\s]+ avoids polynomial backtracking)
    text = _URL_PATTERN.sub('', text)

    # Remove email addresses (negated classes prevent overlapping \S+ ReDoS)
    text = _EMAIL_PATTERN.sub('', text)

    # Remove special characters but keep spaces (avoid regex - CodeQL py/polynomial-redos)
    text = "".join(c if (c.isalnum() or c == "_" or c.isspace()) else " " for c in text)

    # Collapse multiple spaces
    text = re.sub(r'\s+', ' ', text)

    return text.strip()
