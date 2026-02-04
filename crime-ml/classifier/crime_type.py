# streetcode-ml/classifier/crime_type.py
"""Crime type multi-label classifier."""

import joblib
import numpy as np
from typing import TypedDict

from .preprocessor import preprocess_text


class CrimeTypeResult(TypedDict):
    crime_types: list[str]
    scores: dict[str, float]


CRIME_TYPE_THRESHOLD = 0.3  # Minimum probability to include a crime type


class CrimeTypeClassifier:
    """Multi-label classifier for crime types."""

    def __init__(self, model_path: str):
        """Load the trained model from joblib file."""
        data = joblib.load(model_path)
        self.model = data['model']
        self.vectorizer = data['vectorizer']
        self.mlb = data['mlb']
        self.classes = list(self.mlb.classes_)

    def classify(self, text: str) -> CrimeTypeResult:
        """Classify text and return crime types with scores.

        Args:
            text: Combined title + body text

        Returns:
            Dict with 'crime_types' (list) and 'scores' (dict)
        """
        cleaned = preprocess_text(text)
        if not cleaned:
            return {"crime_types": [], "scores": {}}

        # Vectorize
        features = self.vectorizer.transform([cleaned])

        # Get probabilities for each label
        # OneVsRestClassifier returns shape (n_samples, n_classes)
        try:
            probabilities = self.model.predict_proba(features)[0]  # Get first sample
            scores = {}
            crime_types = []

            for i, cls in enumerate(self.classes):
                prob = float(probabilities[i])
                scores[cls] = prob
                if prob >= CRIME_TYPE_THRESHOLD:
                    crime_types.append(cls)

            return {"crime_types": crime_types, "scores": scores}
        except AttributeError:
            # Fallback to binary predict
            predictions = self.model.predict(features)[0]
            crime_types = [cls for i, cls in enumerate(self.classes) if predictions[i]]
            scores = {cls: 1.0 if cls in crime_types else 0.0 for cls in self.classes}
            return {"crime_types": crime_types, "scores": scores}
