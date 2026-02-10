#!/usr/bin/env python3
"""
Mining ML Classifier - Training and Export
Creates placeholder models for mining classification.
Run with real training data later to produce production models.
"""

from pathlib import Path

import joblib
import numpy as np
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.linear_model import LogisticRegression
from sklearn.multiclass import OneVsRestClassifier
from sklearn.preprocessing import MultiLabelBinarizer

RELEVANCE_CLASSES = ['core_mining', 'peripheral_mining', 'not_mining']
MINING_STAGE_CLASSES = ['exploration', 'development', 'production', 'unspecified']
COMMODITY_CLASSES = [
    'gold', 'copper', 'lithium', 'nickel', 'uranium',
    'iron_ore', 'rare_earths', 'other',
]
LOCATION_CLASSES = ['local_canada', 'national_canada', 'international', 'not_specified']

# Synthetic samples for placeholder training (enough to fit vectorizer)
SYNTHETIC_TEXTS = [
    "gold mining exploration in Ontario Canada drill results assay grade",
    "copper production at mine smelter refinery concentrate extraction",
    "lithium nickel rare earths battery metals development project",
    "uranium iron ore mineral deposit reserves tonnes resource",
    "mining industry news business commodity prices market",
    "Toronto Vancouver Sudbury Timmins Canadian mining",
    "international mining project overseas investment",
    "not mining related news article general content",
]


def create_placeholder_relevance():
    """Create placeholder relevance model."""
    texts = SYNTHETIC_TEXTS * 3
    labels = ['core_mining'] * 6 + ['peripheral_mining'] * 6 + ['not_mining'] * 12
    vectorizer = TfidfVectorizer(max_features=500, ngram_range=(1, 2))
    X = vectorizer.fit_transform(texts)
    model = LogisticRegression(max_iter=500, random_state=42)
    model.fit(X, labels)
    return {
        'model': model,
        'vectorizer': vectorizer,
        'classes': RELEVANCE_CLASSES,
    }


def create_placeholder_mining_stage():
    """Create placeholder mining stage model."""
    texts = SYNTHETIC_TEXTS * 4
    labels = ['exploration'] * 8 + ['development'] * 8 + ['production'] * 8 + ['unspecified'] * 8
    vectorizer = TfidfVectorizer(max_features=500, ngram_range=(1, 2))
    X = vectorizer.fit_transform(texts)
    model = LogisticRegression(max_iter=500, random_state=42)
    model.fit(X, labels)
    return {
        'model': model,
        'vectorizer': vectorizer,
        'classes': MINING_STAGE_CLASSES,
    }


def create_placeholder_commodity():
    """Create placeholder commodity multi-label model."""
    texts = SYNTHETIC_TEXTS * 4
    labels_list = [
        ['gold'], ['copper'], ['lithium'], ['nickel'],
        ['uranium'], ['iron_ore'], ['rare_earths'], ['other'],
        ['gold', 'copper'], ['lithium', 'nickel'],
    ] * 3
    mlb = MultiLabelBinarizer(classes=COMMODITY_CLASSES)
    y = mlb.fit_transform(labels_list)
    vectorizer = TfidfVectorizer(max_features=500, ngram_range=(1, 2))
    X = vectorizer.fit_transform(texts[:len(labels_list)])
    model = OneVsRestClassifier(LogisticRegression(max_iter=500, random_state=42))
    model.fit(X, y)
    return {
        'model': model,
        'vectorizer': vectorizer,
        'mlb': mlb,
    }


def create_placeholder_location():
    """Create placeholder location model."""
    texts = SYNTHETIC_TEXTS * 4
    labels = ['local_canada'] * 8 + ['national_canada'] * 8 + ['international'] * 8 + ['not_specified'] * 8
    vectorizer = TfidfVectorizer(max_features=500, ngram_range=(1, 2))
    X = vectorizer.fit_transform(texts)
    model = LogisticRegression(max_iter=500, random_state=42)
    model.fit(X, labels)
    return {
        'model': model,
        'vectorizer': vectorizer,
        'classes': LOCATION_CLASSES,
    }


def main():
    """Export placeholder models to joblib files."""
    output_dir = Path(__file__).parent / 'models'
    output_dir.mkdir(exist_ok=True)

    joblib.dump(create_placeholder_relevance(), output_dir / 'relevance.joblib')
    print("Exported relevance.joblib")

    joblib.dump(create_placeholder_mining_stage(), output_dir / 'mining_stage.joblib')
    print("Exported mining_stage.joblib")

    joblib.dump(create_placeholder_commodity(), output_dir / 'commodity.joblib')
    print("Exported commodity.joblib")

    joblib.dump(create_placeholder_location(), output_dir / 'location.joblib')
    print("Exported location.joblib")

    print("Done - Models exported to", output_dir)


if __name__ == '__main__':
    main()
