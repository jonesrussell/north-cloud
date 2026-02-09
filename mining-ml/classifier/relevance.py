"""Mining relevance classifier (3-class)."""

import joblib
import numpy as np
from typing import TypedDict

from .preprocessor import preprocess_text


class RelevanceResult(TypedDict):
    relevance: str
    confidence: float


class RelevanceClassifier:
    """Classifies articles into core_mining, peripheral_mining, or not_mining."""

    def __init__(self, model_path: str):
        """Load the trained model from joblib file."""
        data = joblib.load(model_path)
        self.model = data['model']
        self.vectorizer = data['vectorizer']
        self.classes = data.get('classes', ['core_mining', 'peripheral_mining', 'not_mining'])

    def classify(self, text: str) -> RelevanceResult:
        """Classify text and return relevance with confidence.

        Args:
            text: Combined title + body text

        Returns:
            Dict with 'relevance' (str) and 'confidence' (float 0-1)
        """
        cleaned = preprocess_text(text)
        if not cleaned:
            return {"relevance": "not_mining", "confidence": 0.5}

        # Vectorize
        features = self.vectorizer.transform([cleaned])

        # Predict with probabilities
        probabilities = self.model.predict_proba(features)[0]
        predicted_idx = np.argmax(probabilities)

        return {
            "relevance": self.classes[predicted_idx],
            "confidence": float(probabilities[predicted_idx]),
        }
