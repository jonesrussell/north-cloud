#!/usr/bin/env python3
"""
Coforge ML Classifier - Training and Export
Creates placeholder models for coforge classification.
Run with real training data later to produce production models.
"""

from pathlib import Path

import joblib
import numpy as np
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.linear_model import LogisticRegression
from sklearn.multiclass import OneVsRestClassifier
from sklearn.preprocessing import MultiLabelBinarizer

RELEVANCE_CLASSES = ['core_coforge', 'peripheral', 'not_relevant']
AUDIENCE_CLASSES = ['developer', 'entrepreneur', 'hybrid']
TOPIC_CLASSES = [
    'framework_release', 'open_source', 'devtools', 'api_sdk',
    'language_update', 'engineering_culture',
    'funding_round', 'acquisition', 'product_launch', 'founder_story',
    'market_analysis', 'partnership',
    'developer_experience', 'ai_ml',
]
INDUSTRY_CLASSES = [
    'ai_ml', 'fintech', 'saas', 'devtools',
    'cloud_infra', 'cybersecurity', 'healthtech', 'other',
]

SYNTHETIC_TEXTS = [
    "AI startup raises Series A funding open source developer tools",
    "React framework release new hooks API TypeScript support",
    "fintech acquisition payment processing SaaS platform enterprise",
    "cloud infrastructure security vulnerability patch kubernetes devops",
    "founder story bootstrapped to million ARR developer experience",
    "open source project CNCF adoption community engineering culture",
    "venture capital funding round seed investment healthtech startup",
    "programming language update Python Rust Go features release",
    "product launch developer SDK API marketplace integration",
    "market analysis SaaS trends developer tools growth forecast",
    "cybersecurity startup partnership enterprise cloud platform",
    "machine learning AI model deployment developer workflow tools",
]


def create_placeholder_relevance():
    """Create placeholder relevance model."""
    texts = SYNTHETIC_TEXTS * 3
    labels = ['core_coforge'] * 12 + ['peripheral'] * 12 + ['not_relevant'] * 12
    vectorizer = TfidfVectorizer(max_features=500, ngram_range=(1, 2))
    X = vectorizer.fit_transform(texts)
    model = LogisticRegression(max_iter=500, random_state=42)
    model.fit(X, labels)
    return {
        'model': model,
        'vectorizer': vectorizer,
        'classes': RELEVANCE_CLASSES,
    }


def create_placeholder_audience():
    """Create placeholder audience model."""
    texts = SYNTHETIC_TEXTS * 3
    labels = ['developer'] * 12 + ['entrepreneur'] * 12 + ['hybrid'] * 12
    vectorizer = TfidfVectorizer(max_features=500, ngram_range=(1, 2))
    X = vectorizer.fit_transform(texts)
    model = LogisticRegression(max_iter=500, random_state=42)
    model.fit(X, labels)
    return {
        'model': model,
        'vectorizer': vectorizer,
        'classes': AUDIENCE_CLASSES,
    }


def create_placeholder_topic():
    """Create placeholder topic multi-label model."""
    texts = SYNTHETIC_TEXTS * 3
    labels_list = [
        ['ai_ml', 'funding_round'], ['framework_release', 'devtools'],
        ['acquisition', 'product_launch'], ['devtools', 'engineering_culture'],
        ['founder_story', 'market_analysis'], ['open_source', 'developer_experience'],
        ['funding_round'], ['language_update'],
        ['api_sdk', 'product_launch'], ['market_analysis'],
        ['partnership'], ['ai_ml', 'devtools'],
    ] * 3
    mlb = MultiLabelBinarizer(classes=TOPIC_CLASSES)
    y = mlb.fit_transform(labels_list)
    vectorizer = TfidfVectorizer(max_features=500, ngram_range=(1, 2))
    X = vectorizer.fit_transform(texts)
    model = OneVsRestClassifier(LogisticRegression(max_iter=500, random_state=42))
    model.fit(X, y)
    return {
        'model': model,
        'vectorizer': vectorizer,
        'mlb': mlb,
    }


def create_placeholder_industry():
    """Create placeholder industry multi-label model."""
    texts = SYNTHETIC_TEXTS * 3
    labels_list = [
        ['ai_ml', 'saas'], ['devtools'], ['fintech', 'saas'],
        ['cloud_infra', 'cybersecurity'], ['saas'],
        ['devtools'], ['healthtech'], ['devtools'],
        ['saas', 'devtools'], ['saas', 'ai_ml'],
        ['cybersecurity', 'cloud_infra'], ['ai_ml', 'devtools'],
    ] * 3
    mlb = MultiLabelBinarizer(classes=INDUSTRY_CLASSES)
    y = mlb.fit_transform(labels_list)
    vectorizer = TfidfVectorizer(max_features=500, ngram_range=(1, 2))
    X = vectorizer.fit_transform(texts)
    model = OneVsRestClassifier(LogisticRegression(max_iter=500, random_state=42))
    model.fit(X, y)
    return {
        'model': model,
        'vectorizer': vectorizer,
        'mlb': mlb,
    }


def main():
    """Export placeholder models to joblib files."""
    output_dir = Path(__file__).parent / 'models'
    output_dir.mkdir(exist_ok=True)

    joblib.dump(create_placeholder_relevance(), output_dir / 'relevance.joblib')
    print("Exported relevance.joblib")

    joblib.dump(create_placeholder_audience(), output_dir / 'audience.joblib')
    print("Exported audience.joblib")

    joblib.dump(create_placeholder_topic(), output_dir / 'topic.joblib')
    print("Exported topic.joblib")

    joblib.dump(create_placeholder_industry(), output_dir / 'industry.joblib')
    print("Exported industry.joblib")

    print("Done - Models exported to", output_dir)


if __name__ == '__main__':
    main()
