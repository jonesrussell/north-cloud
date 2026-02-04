# streetcode-ml/classifier/location.py
"""Location specificity classifier (4-class)."""

import joblib
import numpy as np
from typing import TypedDict

from .preprocessor import preprocess_text


class LocationResult(TypedDict):
    location: str
    confidence: float


class LocationClassifier:
    """Classifies articles by location specificity."""

    def __init__(self, model_path: str):
        """Load the trained model from joblib file."""
        data = joblib.load(model_path)
        self.model = data['model']
        self.vectorizer = data['vectorizer']
        self.classes = data.get('classes', ['local_canada', 'national_canada', 'international', 'not_specified'])

    def classify(self, text: str) -> LocationResult:
        """Classify text and return location with confidence.

        Args:
            text: Combined title + body text

        Returns:
            Dict with 'location' (str) and 'confidence' (float 0-1)
        """
        cleaned = preprocess_text(text)
        if not cleaned:
            return {"location": "not_specified", "confidence": 0.5}

        # Vectorize
        features = self.vectorizer.transform([cleaned])

        # Predict with probabilities
        probabilities = self.model.predict_proba(features)[0]
        predicted_idx = np.argmax(probabilities)

        return {
            "location": self.classes[predicted_idx],
            "confidence": float(probabilities[predicted_idx]),
        }
