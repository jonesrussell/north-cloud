"""Coforge audience classifier (3-class)."""

import joblib
import numpy as np
from typing import TypedDict

from .preprocessor import preprocess_text


class AudienceResult(TypedDict):
    audience: str
    confidence: float


class AudienceClassifier:
    """Classifies articles by target audience: developer, entrepreneur, or hybrid."""

    def __init__(self, model_path: str):
        """Load the trained model from joblib file."""
        data = joblib.load(model_path)
        self.model = data['model']
        self.vectorizer = data['vectorizer']
        self.classes = data.get('classes', ['developer', 'entrepreneur', 'hybrid'])

    def classify(self, text: str) -> AudienceResult:
        """Classify text and return audience with confidence."""
        cleaned = preprocess_text(text)
        if not cleaned:
            return {"audience": "hybrid", "confidence": 0.5}

        features = self.vectorizer.transform([cleaned])
        probabilities = self.model.predict_proba(features)[0]
        predicted_idx = np.argmax(probabilities)

        return {
            "audience": self.classes[predicted_idx],
            "confidence": float(probabilities[predicted_idx]),
        }
