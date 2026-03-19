"""Commodity multi-label classifier."""

import joblib
import numpy as np
from typing import TypedDict

from .preprocessor import preprocess_text


class CommodityResult(TypedDict):
    commodities: list[str]
    scores: dict[str, float]


COMMODITY_THRESHOLD = 0.3  # Minimum probability to include a commodity

COMMODITY_CLASSES = [
    'gold', 'copper', 'lithium', 'nickel', 'uranium',
    'iron_ore', 'rare_earths', 'other',
]


class CommodityClassifier:
    """Multi-label classifier for mining commodities."""

    def __init__(self, model_path: str):
        """Load the trained model from joblib file."""
        data = joblib.load(model_path)
        self.model = data['model']
        self.vectorizer = data['vectorizer']
        self.mlb = data.get('mlb')
        if self.mlb is not None:
            self.classes = list(self.mlb.classes_)
        else:
            self.classes = COMMODITY_CLASSES.copy()

    def classify(self, text: str) -> CommodityResult:
        """Classify text and return commodities with scores.

        Args:
            text: Combined title + body text

        Returns:
            Dict with 'commodities' (list) and 'scores' (dict)
        """
        cleaned = preprocess_text(text)
        if not cleaned:
            return {"commodities": [], "scores": {}}

        # Vectorize
        features = self.vectorizer.transform([cleaned])

        # Get probabilities for each label
        try:
            probabilities = self.model.predict_proba(features)[0]
            scores = {}
            commodities = []

            for i, cls in enumerate(self.classes):
                prob = float(probabilities[i])
                scores[cls] = prob
                if prob >= COMMODITY_THRESHOLD:
                    commodities.append(cls)

            return {"commodities": commodities, "scores": scores}
        except AttributeError:
            # Fallback to binary predict
            predictions = self.model.predict(features)[0]
            commodities = [cls for i, cls in enumerate(self.classes) if predictions[i]]
            scores = {cls: 1.0 if cls in commodities else 0.0 for cls in self.classes}
            return {"commodities": commodities, "scores": scores}
