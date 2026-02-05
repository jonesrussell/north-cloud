# crime-ml/classifier/preprocessor.py
"""Text preprocessing for ML classification."""

import re
from typing import Optional

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

    # Lowercase
    text = text.lower()

    # Remove URLs (single [^\s]+ avoids polynomial backtracking)
    text = _URL_PATTERN.sub('', text)

    # Remove email addresses (negated classes prevent overlapping \S+ ReDoS)
    text = _EMAIL_PATTERN.sub('', text)

    # Remove special characters but keep spaces
    text = re.sub(r'[^\w\s]', ' ', text)

    # Collapse multiple spaces
    text = re.sub(r'\s+', ' ', text)

    return text.strip()
