"""Mining stage classifier (4-class)."""

import joblib
import numpy as np
from typing import TypedDict

from .preprocessor import preprocess_text


class MiningStageResult(TypedDict):
    mining_stage: str
    confidence: float


class MiningStageClassifier:
    """Classifies articles by mining stage: exploration, development, production, unspecified."""

    def __init__(self, model_path: str):
        """Load the trained model from joblib file."""
        data = joblib.load(model_path)
        self.model = data['model']
        self.vectorizer = data['vectorizer']
        self.classes = data.get(
            'classes',
            ['exploration', 'development', 'production', 'unspecified'],
        )

    def classify(self, text: str) -> MiningStageResult:
        """Classify text and return mining stage with confidence.

        Args:
            text: Combined title + body text

        Returns:
            Dict with 'mining_stage' (str) and 'confidence' (float 0-1)
        """
        cleaned = preprocess_text(text)
        if not cleaned:
            return {"mining_stage": "unspecified", "confidence": 0.5}

        # Vectorize
        features = self.vectorizer.transform([cleaned])

        # Predict with probabilities
        probabilities = self.model.predict_proba(features)[0]
        predicted_idx = np.argmax(probabilities)

        return {
            "mining_stage": self.classes[predicted_idx],
            "confidence": float(probabilities[predicted_idx]),
        }
