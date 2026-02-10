"""Coforge topic multi-label classifier."""

import joblib
import numpy as np
from typing import TypedDict

from .preprocessor import preprocess_text


class TopicResult(TypedDict):
    topics: list[str]
    scores: dict[str, float]


TOPIC_THRESHOLD = 0.3

TOPIC_CLASSES = [
    'framework_release', 'open_source', 'devtools', 'api_sdk',
    'language_update', 'engineering_culture',
    'funding_round', 'acquisition', 'product_launch', 'founder_story',
    'market_analysis', 'partnership',
    'developer_experience', 'ai_ml',
]


def _sigmoid(x: float) -> float:
    """Normalize decision_function scores to 0-1 range."""
    return 1.0 / (1.0 + np.exp(-x))


class TopicClassifier:
    """Multi-label classifier for coforge content topics."""

    def __init__(self, model_path: str):
        """Load the trained model from joblib file."""
        data = joblib.load(model_path)
        self.model = data['model']
        self.vectorizer = data['vectorizer']
        self.mlb = data.get('mlb')
        if self.mlb is not None:
            self.classes = list(self.mlb.classes_)
        else:
            self.classes = TOPIC_CLASSES.copy()

    def classify(self, text: str) -> TopicResult:
        """Classify text and return topics with scores."""
        cleaned = preprocess_text(text)
        if not cleaned:
            return {"topics": [], "scores": {}}

        features = self.vectorizer.transform([cleaned])

        try:
            probabilities = self.model.predict_proba(features)[0]
            scores = {}
            topics = []

            for i, cls in enumerate(self.classes):
                prob = float(probabilities[i])
                scores[cls] = prob
                if prob >= TOPIC_THRESHOLD:
                    topics.append(cls)

            return {"topics": topics, "scores": scores}
        except AttributeError:
            try:
                decision_scores = self.model.decision_function(features)[0]
                scores = {}
                topics = []

                for i, cls in enumerate(self.classes):
                    score = float(_sigmoid(decision_scores[i]))
                    scores[cls] = score
                    if score >= TOPIC_THRESHOLD:
                        topics.append(cls)

                return {"topics": topics, "scores": scores}
            except AttributeError:
                predictions = self.model.predict(features)[0]
                topics = [cls for i, cls in enumerate(self.classes) if predictions[i]]
                scores = {cls: 1.0 if cls in topics else 0.0 for cls in self.classes}
                return {"topics": topics, "scores": scores}
