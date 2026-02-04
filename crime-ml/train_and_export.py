#!/usr/bin/env python3
"""
StreetCode.net Classifier - ML Training and Export
Trains models and exports to joblib files for production use.
"""

import json
import re
import warnings
from collections import Counter
from pathlib import Path

import joblib
import numpy as np
import pandas as pd
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.linear_model import LogisticRegression
from sklearn.svm import LinearSVC
from sklearn.multiclass import OneVsRestClassifier
from sklearn.model_selection import train_test_split
from sklearn.metrics import (
    classification_report,
    f1_score,
    precision_score,
    recall_score,
)
from sklearn.preprocessing import MultiLabelBinarizer

warnings.filterwarnings('ignore')


def load_data(classified_path: str, raw_path: str = None) -> pd.DataFrame:
    """Load classified articles and join with raw text if available."""
    classified_records = []
    with open(classified_path, 'r') as f:
        for line in f:
            classified_records.append(json.loads(line))

    df = pd.DataFrame(classified_records)
    print(f"Loaded {len(df)} classified articles")

    if raw_path and Path(raw_path).exists():
        raw_records = []
        with open(raw_path, 'r') as f:
            for line in f:
                raw_records.append(json.loads(line))
        raw_df = pd.DataFrame(raw_records)
        print(f"Loaded {len(raw_df)} raw articles with text")
        df = df.merge(raw_df[['id', 'raw_text']], on='id', how='left')
    else:
        df['raw_text'] = ''
        print("WARNING: No raw text available, using title only")

    return df


def preprocess_text(text: str) -> str:
    """Clean and normalize text for vectorization."""
    if not text:
        return ""
    text = text.lower()
    text = re.sub(r'https?://\S+', '', text)
    text = re.sub(r'\S+@\S+', '', text)
    text = re.sub(r'[^\w\s]', ' ', text)
    text = re.sub(r'\s+', ' ', text)
    return text.strip()


def prepare_features(df: pd.DataFrame) -> pd.DataFrame:
    """Prepare text features from title and body."""
    df = df.copy()
    df['text'] = df['title'].fillna('') + ' ' + df['raw_text'].fillna('').str[:500]
    df['text_clean'] = df['text'].apply(preprocess_text)
    df = df[df['text_clean'].str.len() > 10]
    print(f"After filtering: {len(df)} articles with valid text")
    return df


def train_relevance_model(df: pd.DataFrame) -> dict:
    """Train street_crime_relevance classifier."""
    print("\n" + "=" * 60)
    print("MODEL 1: STREET CRIME RELEVANCE (3-class)")
    print("=" * 60)

    y = df['new_relevance'].values

    print("\nClass distribution:")
    for cls, count in Counter(y).most_common():
        print(f"  {cls}: {count} ({count/len(y)*100:.1f}%)")

    vectorizer = TfidfVectorizer(
        max_features=5000,
        ngram_range=(1, 2),
        min_df=2,
        max_df=0.95,
        stop_words='english',
    )
    X = vectorizer.fit_transform(df['text_clean'])

    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.2, random_state=42, stratify=y
    )

    # Train Logistic Regression (best for export - supports predict_proba)
    model = LogisticRegression(
        max_iter=1000,
        class_weight='balanced',
        random_state=42
    )
    model.fit(X_train, y_train)

    y_pred = model.predict(X_test)
    print("\nClassification Report:")
    print(classification_report(y_test, y_pred, zero_division=0))

    return {
        'model': model,
        'vectorizer': vectorizer,
        'f1_macro': f1_score(y_test, y_pred, average='macro', zero_division=0),
        'precision_core': precision_score(y_test, y_pred, labels=['core_street_crime'], average='micro', zero_division=0),
        'recall_core': recall_score(y_test, y_pred, labels=['core_street_crime'], average='micro', zero_division=0),
    }


def train_crime_type_model(df: pd.DataFrame) -> dict:
    """Train crime_type multi-label classifier."""
    print("\n" + "=" * 60)
    print("MODEL 2: CRIME TYPE (multi-label)")
    print("=" * 60)

    crime_df = df[df['new_relevance'].isin(['core_street_crime', 'peripheral_crime'])].copy()
    print(f"\nCrime articles: {len(crime_df)}")

    if len(crime_df) < 50:
        print("WARNING: Not enough crime articles for reliable multi-label training")
        return {}

    crime_types = ['violent_crime', 'property_crime', 'drug_crime', 'gang_violence',
                   'organized_crime', 'criminal_justice', 'other_crime']

    mlb = MultiLabelBinarizer(classes=crime_types)
    y = mlb.fit_transform(crime_df['new_crime_types'])

    print("\nLabel distribution:")
    for i, ct in enumerate(crime_types):
        count = y[:, i].sum()
        print(f"  {ct}: {count} ({count/len(y)*100:.1f}%)")

    vectorizer = TfidfVectorizer(
        max_features=3000,
        ngram_range=(1, 2),
        min_df=2,
        max_df=0.95,
        stop_words='english',
    )
    X = vectorizer.fit_transform(crime_df['text_clean'])

    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.2, random_state=42
    )

    model = OneVsRestClassifier(
        LogisticRegression(max_iter=1000, class_weight='balanced', random_state=42)
    )
    model.fit(X_train, y_train)

    y_pred = model.predict(X_test)
    print("\nPer-label metrics:")
    print(f"{'Label':25s} {'Precision':>10s} {'Recall':>10s} {'F1':>10s}")
    print("-" * 55)

    for i, ct in enumerate(crime_types):
        if y_test[:, i].sum() > 0:
            p = precision_score(y_test[:, i], y_pred[:, i], zero_division=0)
            r = recall_score(y_test[:, i], y_pred[:, i], zero_division=0)
            f1 = f1_score(y_test[:, i], y_pred[:, i], zero_division=0)
            print(f"{ct:25s} {p:10.2%} {r:10.2%} {f1:10.2%}")

    return {
        'model': model,
        'vectorizer': vectorizer,
        'mlb': mlb,
    }


def train_location_model(df: pd.DataFrame) -> dict:
    """Train location_specificity classifier."""
    print("\n" + "=" * 60)
    print("MODEL 3: LOCATION SPECIFICITY (4-class)")
    print("=" * 60)

    y = df['new_location'].values

    print("\nClass distribution:")
    for cls, count in Counter(y).most_common():
        print(f"  {cls}: {count} ({count/len(y)*100:.1f}%)")

    vectorizer = TfidfVectorizer(
        max_features=3000,
        ngram_range=(1, 2),
        min_df=2,
        max_df=0.95,
        stop_words='english',
    )
    X = vectorizer.fit_transform(df['text_clean'])

    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.2, random_state=42, stratify=y
    )

    model = LogisticRegression(
        max_iter=1000,
        class_weight='balanced',
        random_state=42
    )
    model.fit(X_train, y_train)

    y_pred = model.predict(X_test)
    print("\nClassification Report:")
    print(classification_report(y_test, y_pred, zero_division=0))

    return {
        'model': model,
        'vectorizer': vectorizer,
    }


def export_models(relevance_results: dict, crime_type_results: dict, location_results: dict, output_dir: str):
    """Export trained models to joblib files."""
    print("\n" + "=" * 60)
    print("EXPORTING MODELS")
    print("=" * 60)

    import os
    os.makedirs(output_dir, exist_ok=True)

    # Export relevance model
    if relevance_results:
        joblib.dump({
            'model': relevance_results['model'],
            'vectorizer': relevance_results['vectorizer'],
            'classes': ['core_street_crime', 'peripheral_crime', 'not_crime'],
        }, f'{output_dir}/relevance.joblib')
        print(f"Exported relevance model")

    # Export crime type model
    if crime_type_results:
        joblib.dump({
            'model': crime_type_results['model'],
            'vectorizer': crime_type_results['vectorizer'],
            'mlb': crime_type_results['mlb'],
        }, f'{output_dir}/crime_type.joblib')
        print("Exported crime_type model")

    # Export location model
    if location_results:
        joblib.dump({
            'model': location_results['model'],
            'vectorizer': location_results['vectorizer'],
            'classes': ['local_canada', 'national_canada', 'international', 'not_specified'],
        }, f'{output_dir}/location.joblib')
        print("Exported location model")


def main():
    # Load data
    classified_path = Path('/home/fsd42/dev/north-cloud/docs/plans/streetcode_classified.jsonl')
    raw_path = '/tmp/streetcode_export.jsonl'
    df = load_data(str(classified_path), raw_path)

    # Prepare features
    df = prepare_features(df)

    # Train models
    relevance_results = train_relevance_model(df)
    crime_type_results = train_crime_type_model(df)
    location_results = train_location_model(df)

    # Export models
    output_dir = str(Path(__file__).parent / 'models')
    export_models(relevance_results, crime_type_results, location_results, output_dir)

    print("\n" + "=" * 60)
    print("DONE - Models exported to", output_dir)
    print("=" * 60)


if __name__ == '__main__':
    main()
