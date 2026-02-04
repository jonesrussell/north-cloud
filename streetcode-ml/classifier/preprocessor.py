# streetcode-ml/classifier/preprocessor.py
"""Text preprocessing for ML classification."""

import re
from typing import Optional


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

    # Remove URLs
    text = re.sub(r'https?://\S+', '', text)

    # Remove email addresses
    text = re.sub(r'\S+@\S+', '', text)

    # Remove special characters but keep spaces
    text = re.sub(r'[^\w\s]', ' ', text)

    # Collapse multiple spaces
    text = re.sub(r'\s+', ' ', text)

    return text.strip()
