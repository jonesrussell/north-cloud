"""Coforge industry multi-label classifier."""

import joblib
import numpy as np
from typing import TypedDict

from .preprocessor import preprocess_text


class IndustryResult(TypedDict):
    industries: list[str]
    scores: dict[str, float]


INDUSTRY_THRESHOLD = 0.3

INDUSTRY_CLASSES = [
    'ai_ml', 'fintech', 'saas', 'devtools',
    'cloud_infra', 'cybersecurity', 'healthtech', 'other',
]


def _sigmoid(x: float) -> float:
    """Normalize decision_function scores to 0-1 range."""
    return 1.0 / (1.0 + np.exp(-x))


class IndustryClassifier:
    """Multi-label classifier for industry verticals."""

    def __init__(self, model_path: str):
        """Load the trained model from joblib file."""
        data = joblib.load(model_path)
        self.model = data['model']
        self.vectorizer = data['vectorizer']
        self.mlb = data.get('mlb')
        if self.mlb is not None:
            self.classes = list(self.mlb.classes_)
        else:
            self.classes = INDUSTRY_CLASSES.copy()

    def classify(self, text: str) -> IndustryResult:
        """Classify text and return industries with scores."""
        cleaned = preprocess_text(text)
        if not cleaned:
            return {"industries": [], "scores": {}}

        features = self.vectorizer.transform([cleaned])

        try:
            probabilities = self.model.predict_proba(features)[0]
            scores = {}
            industries = []

            for i, cls in enumerate(self.classes):
                prob = float(probabilities[i])
                scores[cls] = prob
                if prob >= INDUSTRY_THRESHOLD:
                    industries.append(cls)

            return {"industries": industries, "scores": scores}
        except AttributeError:
            try:
                decision_scores = self.model.decision_function(features)[0]
                scores = {}
                industries = []

                for i, cls in enumerate(self.classes):
                    score = float(_sigmoid(decision_scores[i]))
                    scores[cls] = score
                    if score >= INDUSTRY_THRESHOLD:
                        industries.append(cls)

                return {"industries": industries, "scores": scores}
            except AttributeError:
                predictions = self.model.predict(features)[0]
                industries = [cls for i, cls in enumerate(self.classes) if predictions[i]]
                scores = {cls: 1.0 if cls in industries else 0.0 for cls in self.classes}
                return {"industries": industries, "scores": scores}
