#!/usr/bin/env python3
"""
StreetCode.net Classifier - ML Training Pipeline
Stage 4: scikit-learn baseline models

Models trained:
1. street_crime_relevance (3-class: core, peripheral, not_crime)
2. crime_type (multi-label: violent, property, drug, gang, organized, criminal_justice, other)
3. location_specificity (4-class: local_canada, national_canada, international, not_specified)
"""

import json
import re
import warnings
from collections import Counter
from pathlib import Path

import numpy as np
import pandas as pd
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.linear_model import LogisticRegression
from sklearn.svm import LinearSVC
from sklearn.multiclass import OneVsRestClassifier
from sklearn.model_selection import train_test_split, cross_val_score
from sklearn.metrics import (
    classification_report,
    confusion_matrix,
    precision_recall_curve,
    f1_score,
    precision_score,
    recall_score,
)
from sklearn.preprocessing import MultiLabelBinarizer

warnings.filterwarnings('ignore')

# =============================================================================
# DATA LOADING
# =============================================================================

def load_data(classified_path: str, raw_path: str = None) -> pd.DataFrame:
    """Load classified articles and join with raw text if available."""
    # Load classified data
    classified_records = []
    with open(classified_path, 'r') as f:
        for line in f:
            classified_records.append(json.loads(line))

    df = pd.DataFrame(classified_records)
    print(f"Loaded {len(df)} classified articles")

    # Try to load raw data for text features
    if raw_path and Path(raw_path).exists():
        raw_records = []
        with open(raw_path, 'r') as f:
            for line in f:
                raw_records.append(json.loads(line))
        raw_df = pd.DataFrame(raw_records)
        print(f"Loaded {len(raw_df)} raw articles with text")

        # Join on id
        df = df.merge(raw_df[['id', 'raw_text']], on='id', how='left')
    else:
        # Use title only if no raw text available
        df['raw_text'] = ''
        print("WARNING: No raw text available, using title only")

    return df


def preprocess_text(text: str) -> str:
    """Clean and normalize text for vectorization."""
    if not text:
        return ""
    # Lowercase
    text = text.lower()
    # Remove URLs
    text = re.sub(r'https?://\S+', '', text)
    # Remove email addresses
    text = re.sub(r'\S+@\S+', '', text)
    # Remove special characters but keep spaces
    text = re.sub(r'[^\w\s]', ' ', text)
    # Collapse multiple spaces
    text = re.sub(r'\s+', ' ', text)
    return text.strip()


def prepare_features(df: pd.DataFrame) -> pd.DataFrame:
    """Prepare text features from title and body."""
    # Combine title with first 500 chars of body for feature extraction
    df = df.copy()
    df['text'] = df['title'].fillna('') + ' ' + df['raw_text'].fillna('').str[:500]
    df['text_clean'] = df['text'].apply(preprocess_text)

    # Filter out empty texts
    df = df[df['text_clean'].str.len() > 10]

    print(f"After filtering: {len(df)} articles with valid text")
    return df


# =============================================================================
# MODEL 1: STREET CRIME RELEVANCE (3-class)
# =============================================================================

def train_relevance_model(df: pd.DataFrame) -> dict:
    """Train and evaluate street_crime_relevance classifier."""
    print("\n" + "=" * 60)
    print("MODEL 1: STREET CRIME RELEVANCE (3-class)")
    print("=" * 60)

    # Prepare labels
    y = df['new_relevance'].values

    # Class distribution
    print("\nClass distribution:")
    for cls, count in Counter(y).most_common():
        print(f"  {cls}: {count} ({count/len(y)*100:.1f}%)")

    # TF-IDF vectorization
    vectorizer = TfidfVectorizer(
        max_features=5000,
        ngram_range=(1, 2),  # Unigrams and bigrams
        min_df=2,
        max_df=0.95,
        stop_words='english',
    )
    X = vectorizer.fit_transform(df['text_clean'])

    print(f"\nFeature matrix: {X.shape}")

    # Train/test split (stratified)
    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.2, random_state=42, stratify=y
    )

    print(f"Train: {X_train.shape[0]}, Test: {X_test.shape[0]}")

    # Train models
    models = {
        'Logistic Regression': LogisticRegression(
            max_iter=1000,
            class_weight='balanced',
            random_state=42
        ),
        'Linear SVM': LinearSVC(
            class_weight='balanced',
            random_state=42,
            max_iter=2000
        ),
    }

    results = {}

    for name, model in models.items():
        print(f"\n--- {name} ---")

        # Train
        model.fit(X_train, y_train)

        # Predict
        y_pred = model.predict(X_test)

        # Metrics
        print("\nClassification Report:")
        print(classification_report(y_test, y_pred, zero_division=0))

        # Confusion matrix
        print("\nConfusion Matrix:")
        cm = confusion_matrix(y_test, y_pred, labels=['core_street_crime', 'peripheral_crime', 'not_crime'])
        print(f"                    Predicted")
        print(f"                    core    periph  not")
        print(f"Actual core         {cm[0][0]:4d}    {cm[0][1]:4d}    {cm[0][2]:4d}")
        print(f"       peripheral   {cm[1][0]:4d}    {cm[1][1]:4d}    {cm[1][2]:4d}")
        print(f"       not_crime    {cm[2][0]:4d}    {cm[2][1]:4d}    {cm[2][2]:4d}")

        # Store results
        results[name] = {
            'model': model,
            'vectorizer': vectorizer,
            'f1_macro': f1_score(y_test, y_pred, average='macro', zero_division=0),
            'precision_core': precision_score(y_test, y_pred, labels=['core_street_crime'], average='micro', zero_division=0),
            'recall_core': recall_score(y_test, y_pred, labels=['core_street_crime'], average='micro', zero_division=0),
        }

        print(f"\nHomepage precision (core_street_crime): {results[name]['precision_core']:.2%}")
        print(f"Homepage recall (core_street_crime): {results[name]['recall_core']:.2%}")

    # Feature importance (from Logistic Regression)
    print("\n--- Top Features for core_street_crime ---")
    lr_model = results['Logistic Regression']['model']
    feature_names = vectorizer.get_feature_names_out()

    # Find the index for core_street_crime class
    core_idx = list(lr_model.classes_).index('core_street_crime')
    coefs = lr_model.coef_[core_idx]

    top_positive = np.argsort(coefs)[-20:][::-1]
    top_negative = np.argsort(coefs)[:10]

    print("\nTop 20 features predicting CORE STREET CRIME:")
    for idx in top_positive:
        print(f"  {feature_names[idx]:30s} {coefs[idx]:+.3f}")

    print("\nTop 10 features predicting NOT CRIME:")
    for idx in top_negative:
        print(f"  {feature_names[idx]:30s} {coefs[idx]:+.3f}")

    return results


# =============================================================================
# MODEL 2: CRIME TYPE (multi-label)
# =============================================================================

def train_crime_type_model(df: pd.DataFrame) -> dict:
    """Train and evaluate crime_type multi-label classifier."""
    print("\n" + "=" * 60)
    print("MODEL 2: CRIME TYPE (multi-label)")
    print("=" * 60)

    # Filter to crime-related articles only
    crime_df = df[df['new_relevance'].isin(['core_street_crime', 'peripheral_crime'])].copy()
    print(f"\nCrime articles: {len(crime_df)}")

    if len(crime_df) < 50:
        print("WARNING: Not enough crime articles for reliable multi-label training")
        return {}

    # Prepare multi-label targets
    crime_types = ['violent_crime', 'property_crime', 'drug_crime', 'gang_violence',
                   'organized_crime', 'criminal_justice', 'other_crime']

    mlb = MultiLabelBinarizer(classes=crime_types)
    y = mlb.fit_transform(crime_df['new_crime_types'])

    print("\nLabel distribution:")
    for i, ct in enumerate(crime_types):
        count = y[:, i].sum()
        print(f"  {ct}: {count} ({count/len(y)*100:.1f}%)")

    # TF-IDF vectorization
    vectorizer = TfidfVectorizer(
        max_features=3000,
        ngram_range=(1, 2),
        min_df=2,
        max_df=0.95,
        stop_words='english',
    )
    X = vectorizer.fit_transform(crime_df['text_clean'])

    # Train/test split
    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.2, random_state=42
    )

    print(f"\nTrain: {X_train.shape[0]}, Test: {X_test.shape[0]}")

    # Train OneVsRest classifier
    model = OneVsRestClassifier(
        LogisticRegression(max_iter=1000, class_weight='balanced', random_state=42)
    )
    model.fit(X_train, y_train)

    # Predict
    y_pred = model.predict(X_test)

    # Per-label metrics
    print("\nPer-label metrics:")
    print(f"{'Label':25s} {'Precision':>10s} {'Recall':>10s} {'F1':>10s} {'Support':>10s}")
    print("-" * 65)

    for i, ct in enumerate(crime_types):
        if y_test[:, i].sum() > 0:
            p = precision_score(y_test[:, i], y_pred[:, i], zero_division=0)
            r = recall_score(y_test[:, i], y_pred[:, i], zero_division=0)
            f1 = f1_score(y_test[:, i], y_pred[:, i], zero_division=0)
            support = y_test[:, i].sum()
            print(f"{ct:25s} {p:10.2%} {r:10.2%} {f1:10.2%} {support:10d}")

    return {
        'model': model,
        'vectorizer': vectorizer,
        'mlb': mlb,
    }


# =============================================================================
# MODEL 3: LOCATION SPECIFICITY (4-class)
# =============================================================================

def train_location_model(df: pd.DataFrame) -> dict:
    """Train and evaluate location_specificity classifier."""
    print("\n" + "=" * 60)
    print("MODEL 3: LOCATION SPECIFICITY (4-class)")
    print("=" * 60)

    y = df['new_location'].values

    print("\nClass distribution:")
    for cls, count in Counter(y).most_common():
        print(f"  {cls}: {count} ({count/len(y)*100:.1f}%)")

    # TF-IDF vectorization
    vectorizer = TfidfVectorizer(
        max_features=3000,
        ngram_range=(1, 2),
        min_df=2,
        max_df=0.95,
        stop_words='english',
    )
    X = vectorizer.fit_transform(df['text_clean'])

    # Train/test split
    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.2, random_state=42, stratify=y
    )

    # Train model
    model = LogisticRegression(
        max_iter=1000,
        class_weight='balanced',
        random_state=42
    )
    model.fit(X_train, y_train)

    # Predict
    y_pred = model.predict(X_test)

    print("\nClassification Report:")
    print(classification_report(y_test, y_pred, zero_division=0))

    return {
        'model': model,
        'vectorizer': vectorizer,
    }


# =============================================================================
# ERROR ANALYSIS
# =============================================================================

def analyze_errors(df: pd.DataFrame, model, vectorizer, y_test, y_pred, X_test_indices):
    """Analyze false positives and false negatives."""
    print("\n" + "=" * 60)
    print("ERROR ANALYSIS")
    print("=" * 60)

    test_df = df.iloc[X_test_indices].copy()
    test_df['predicted'] = y_pred
    test_df['actual'] = y_test

    # False positives: predicted core, actual not_crime
    fp = test_df[(test_df['predicted'] == 'core_street_crime') &
                  (test_df['actual'] == 'not_crime')]

    print(f"\n--- FALSE POSITIVES (predicted core, actual not_crime): {len(fp)} ---")
    for _, row in fp.head(10).iterrows():
        print(f"  Title: {row['title'][:70]}")
        print(f"  Actual: {row['actual']}, Predicted: {row['predicted']}")
        print()

    # False negatives: predicted not_crime, actual core
    fn = test_df[(test_df['predicted'] == 'not_crime') &
                  (test_df['actual'] == 'core_street_crime')]

    print(f"\n--- FALSE NEGATIVES (predicted not_crime, actual core): {len(fn)} ---")
    for _, row in fn.head(10).iterrows():
        print(f"  Title: {row['title'][:70]}")
        print(f"  Actual: {row['actual']}, Predicted: {row['predicted']}")
        print()


# =============================================================================
# MAIN
# =============================================================================

def main():
    # Load data
    classified_path = Path(__file__).parent / 'streetcode_classified.jsonl'
    raw_path = '/tmp/streetcode_export.jsonl'  # Original export with raw_text
    df = load_data(str(classified_path), raw_path)

    # Prepare features
    df = prepare_features(df)

    # Train models
    relevance_results = train_relevance_model(df)
    crime_type_results = train_crime_type_model(df)
    location_results = train_location_model(df)

    # Summary
    print("\n" + "=" * 60)
    print("SUMMARY")
    print("=" * 60)

    print("\nModel 1 - Street Crime Relevance:")
    for name, res in relevance_results.items():
        print(f"  {name}:")
        print(f"    F1 (macro): {res['f1_macro']:.2%}")
        print(f"    Core precision: {res['precision_core']:.2%}")
        print(f"    Core recall: {res['recall_core']:.2%}")

    print("\n" + "-" * 60)
    print("RECOMMENDATIONS")
    print("-" * 60)

    best_model = max(relevance_results.items(), key=lambda x: x[1]['f1_macro'])
    print(f"\nBest model: {best_model[0]}")
    print(f"  - Use confidence threshold >= 0.7 for homepage")
    print(f"  - Fall back to rules when confidence < 0.5")
    print(f"  - Consider transformer if core precision < 90%")


if __name__ == '__main__':
    main()
