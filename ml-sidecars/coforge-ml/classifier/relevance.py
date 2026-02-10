"""Coforge relevance classifier (3-class)."""

import joblib
import numpy as np
from typing import TypedDict

from .preprocessor import preprocess_text


class RelevanceResult(TypedDict):
    relevance: str
    confidence: float


class RelevanceClassifier:
    """Classifies articles into core_coforge, peripheral, or not_relevant."""

    def __init__(self, model_path: str):
        """Load the trained model from joblib file."""
        data = joblib.load(model_path)
        self.model = data['model']
        self.vectorizer = data['vectorizer']
        self.classes = data.get('classes', ['core_coforge', 'peripheral', 'not_relevant'])

    def classify(self, text: str) -> RelevanceResult:
        """Classify text and return relevance with confidence."""
        cleaned = preprocess_text(text)
        if not cleaned:
            return {"relevance": "not_relevant", "confidence": 0.5}

        features = self.vectorizer.transform([cleaned])
        probabilities = self.model.predict_proba(features)[0]
        predicted_idx = np.argmax(probabilities)

        return {
            "relevance": self.classes[predicted_idx],
            "confidence": float(probabilities[predicted_idx]),
        }
