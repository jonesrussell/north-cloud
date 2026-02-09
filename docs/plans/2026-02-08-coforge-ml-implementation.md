# Coforge-ML Sidecar Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Python FastAPI ML sidecar for coforge.xyz that classifies content along four axes (relevance, audience, topic, industry), then integrate it into the classifier Go service, Elasticsearch mappings, and Docker stack.

**Architecture:** Four scikit-learn models (relevance, audience, topic, industry) served via FastAPI, consumed by a Go HTTP client in the classifier service using the same hybrid rules+ML pattern as crime-ml and mining-ml. Establishes gold-standard patterns for future backporting.

**Tech Stack:** Python 3.11, FastAPI, scikit-learn, joblib | Go 1.24+, Gin, Elasticsearch 8, Docker

**Design doc:** `docs/plans/2026-02-08-coforge-ml-design.md`

---

## Task 1: Create coforge-ml Python Package Scaffolding

Create the base Python package with preprocessor, requirements, Dockerfile, and gitignore.

**Files:**
- Create: `coforge-ml/classifier/__init__.py`
- Create: `coforge-ml/classifier/preprocessor.py`
- Create: `coforge-ml/requirements.txt`
- Create: `coforge-ml/Dockerfile`
- Create: `coforge-ml/.gitignore`

**Step 1: Create `coforge-ml/classifier/__init__.py`**

```python
"""Coforge ML Classifier package."""
```

**Step 2: Create `coforge-ml/classifier/preprocessor.py`**

Copy the mining-ml preprocessor verbatim — this is the gold-standard version (character-based, ReDoS-safe):

```python
# coforge-ml/classifier/preprocessor.py
"""Text preprocessing for ML classification."""

import re
from typing import Optional

# Cap input length to bound regex work and avoid ReDoS on adversarial input (CodeQL py/polynomial-redos).
MAX_PREPROCESS_LENGTH = 1_000_000

# ReDoS-safe patterns: avoid ambiguous greedy repetition (e.g. \S+@\S+).
# URL: one [^\s]+ so no backtracking overlap.
_URL_PATTERN = re.compile(r'https?://[^\s]+')
# Email: [^\s@]+ before and after @ so the two parts cannot match the same span.
_EMAIL_PATTERN = re.compile(r'[^\s@]+@[^\s@]+')


def preprocess_text(text: Optional[str]) -> str:
    """Clean and normalize text for vectorization.

    Args:
        text: Raw text input, may be None

    Returns:
        Cleaned, lowercase text with URLs/emails removed
    """
    if not text:
        return ""

    if len(text) > MAX_PREPROCESS_LENGTH:
        text = text[:MAX_PREPROCESS_LENGTH]

    # Lowercase
    text = text.lower()

    # Remove URLs (single [^\s]+ avoids polynomial backtracking)
    text = _URL_PATTERN.sub('', text)

    # Remove email addresses (negated classes prevent overlapping \S+ ReDoS)
    text = _EMAIL_PATTERN.sub('', text)

    # Remove special characters but keep spaces (avoid regex - CodeQL py/polynomial-redos)
    text = "".join(c if (c.isalnum() or c == "_" or c.isspace()) else " " for c in text)

    # Collapse multiple spaces
    text = re.sub(r'\s+', ' ', text)

    return text.strip()
```

**Step 3: Create `coforge-ml/requirements.txt`**

```
fastapi>=0.109.2
uvicorn>=0.27.1
pydantic>=2.6.1
scikit-learn>=1.4.0
joblib>=1.3.2
numpy>=1.26.4
pytest>=8.0.0
```

**Step 4: Create `coforge-ml/Dockerfile`**

```dockerfile
FROM python:3.11-slim

WORKDIR /app

# Install curl for healthcheck
RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

# Build placeholder models if not present
RUN python train_and_export.py 2>/dev/null || true

EXPOSE 8078

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8078"]
```

**Step 5: Create `coforge-ml/.gitignore`**

```
.venv/
__pycache__/
*.pyc
```

**Step 6: Commit**

```bash
git add coforge-ml/classifier/__init__.py coforge-ml/classifier/preprocessor.py coforge-ml/requirements.txt coforge-ml/Dockerfile coforge-ml/.gitignore
git commit -m "feat(coforge-ml): scaffold Python package with preprocessor and Dockerfile"
```

---

## Task 2: Create Preprocessor Tests

**Files:**
- Create: `coforge-ml/tests/__init__.py`
- Create: `coforge-ml/tests/test_preprocessor.py`

**Step 1: Create `coforge-ml/tests/__init__.py`**

Empty file.

**Step 2: Create `coforge-ml/tests/test_preprocessor.py`**

```python
import pytest
from classifier.preprocessor import preprocess_text


class TestPreprocessText:
    def test_lowercases_text(self):
        result = preprocess_text("AI Startup Raises SERIES A")
        assert "ai startup raises series a" in result

    def test_removes_urls(self):
        result = preprocess_text("Story at https://example.com/news")
        assert "https://" not in result
        assert "example.com" not in result

    def test_removes_emails(self):
        result = preprocess_text("Contact news@example.com for tips")
        assert "@" not in result

    def test_collapses_whitespace(self):
        result = preprocess_text("Multiple   spaces   here")
        assert "  " not in result

    def test_handles_empty_string(self):
        result = preprocess_text("")
        assert result == ""

    def test_handles_none(self):
        result = preprocess_text(None)
        assert result == ""
```

**Step 3: Run tests**

Run: `cd coforge-ml && python -m pytest tests/test_preprocessor.py -v`
Expected: PASS (all 6 tests)

**Step 4: Commit**

```bash
git add coforge-ml/tests/
git commit -m "test(coforge-ml): add preprocessor tests"
```

---

## Task 3: Create Four Classifier Modules

**Files:**
- Create: `coforge-ml/classifier/relevance.py`
- Create: `coforge-ml/classifier/audience.py`
- Create: `coforge-ml/classifier/topic.py`
- Create: `coforge-ml/classifier/industry.py`

**Step 1: Create `coforge-ml/classifier/relevance.py`**

3-class single-label classifier. Follows mining-ml/classifier/relevance.py pattern exactly.

```python
"""Coforge relevance classifier (3-class)."""

import joblib
import numpy as np
from typing import TypedDict

from .preprocessor import preprocess_text


class RelevanceResult(TypedDict):
    relevance: str
    confidence: float


class RelevanceClassifier:
    """Classifies articles into core_coforge, peripheral, or not_relevant."""

    def __init__(self, model_path: str):
        """Load the trained model from joblib file."""
        data = joblib.load(model_path)
        self.model = data['model']
        self.vectorizer = data['vectorizer']
        self.classes = data.get('classes', ['core_coforge', 'peripheral', 'not_relevant'])

    def classify(self, text: str) -> RelevanceResult:
        """Classify text and return relevance with confidence.

        Args:
            text: Combined title + body text

        Returns:
            Dict with 'relevance' (str) and 'confidence' (float 0-1)
        """
        cleaned = preprocess_text(text)
        if not cleaned:
            return {"relevance": "not_relevant", "confidence": 0.5}

        features = self.vectorizer.transform([cleaned])
        probabilities = self.model.predict_proba(features)[0]
        predicted_idx = np.argmax(probabilities)

        return {
            "relevance": self.classes[predicted_idx],
            "confidence": float(probabilities[predicted_idx]),
        }
```

**Step 2: Create `coforge-ml/classifier/audience.py`**

3-class single-label classifier. Same pattern as relevance, different labels.

```python
"""Coforge audience classifier (3-class)."""

import joblib
import numpy as np
from typing import TypedDict

from .preprocessor import preprocess_text


class AudienceResult(TypedDict):
    audience: str
    confidence: float


class AudienceClassifier:
    """Classifies articles by target audience: developer, entrepreneur, or hybrid."""

    def __init__(self, model_path: str):
        """Load the trained model from joblib file."""
        data = joblib.load(model_path)
        self.model = data['model']
        self.vectorizer = data['vectorizer']
        self.classes = data.get('classes', ['developer', 'entrepreneur', 'hybrid'])

    def classify(self, text: str) -> AudienceResult:
        """Classify text and return audience with confidence.

        Args:
            text: Combined title + body text

        Returns:
            Dict with 'audience' (str) and 'confidence' (float 0-1)
        """
        cleaned = preprocess_text(text)
        if not cleaned:
            return {"audience": "hybrid", "confidence": 0.5}

        features = self.vectorizer.transform([cleaned])
        probabilities = self.model.predict_proba(features)[0]
        predicted_idx = np.argmax(probabilities)

        return {
            "audience": self.classes[predicted_idx],
            "confidence": float(probabilities[predicted_idx]),
        }
```

**Step 3: Create `coforge-ml/classifier/topic.py`**

Multi-label classifier. Follows mining-ml/classifier/commodity.py pattern with sigmoid fallback (gold-standard fix).

```python
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
        """Classify text and return topics with scores.

        Args:
            text: Combined title + body text

        Returns:
            Dict with 'topics' (list) and 'scores' (dict)
        """
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
            # Fallback: use decision_function with sigmoid normalization
            # (gold-standard fix — not raw binary like mining-ml/crime-ml)
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
                # Last resort: binary predict
                predictions = self.model.predict(features)[0]
                topics = [cls for i, cls in enumerate(self.classes) if predictions[i]]
                scores = {cls: 1.0 if cls in topics else 0.0 for cls in self.classes}
                return {"topics": topics, "scores": scores}
```

**Step 4: Create `coforge-ml/classifier/industry.py`**

Multi-label classifier. Same pattern as topic.py, different labels.

```python
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
        """Classify text and return industries with scores.

        Args:
            text: Combined title + body text

        Returns:
            Dict with 'industries' (list) and 'scores' (dict)
        """
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
```

**Step 5: Commit**

```bash
git add coforge-ml/classifier/relevance.py coforge-ml/classifier/audience.py coforge-ml/classifier/topic.py coforge-ml/classifier/industry.py
git commit -m "feat(coforge-ml): add relevance, audience, topic, and industry classifiers"
```

---

## Task 4: Create Training Script and Placeholder Models

**Files:**
- Create: `coforge-ml/train_and_export.py`

**Step 1: Create `coforge-ml/train_and_export.py`**

Follows mining-ml/train_and_export.py pattern — synthetic data for placeholder models.

```python
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
```

**Step 2: Generate placeholder models**

Run: `cd coforge-ml && python train_and_export.py`
Expected: 4 `.joblib` files in `coforge-ml/models/`

**Step 3: Commit (models excluded by .gitignore — only commit script)**

```bash
git add coforge-ml/train_and_export.py
git commit -m "feat(coforge-ml): add training script with placeholder model generation"
```

---

## Task 5: Create FastAPI Server (main.py)

**Files:**
- Create: `coforge-ml/main.py`

**Step 1: Create `coforge-ml/main.py`**

Follows mining-ml/main.py pattern with gold-standard `models_loaded` guard and date-prefixed version.

```python
"""Coforge ML Classifier - FastAPI Server."""

import time
from contextlib import asynccontextmanager
from typing import Optional

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

from classifier.relevance import RelevanceClassifier
from classifier.audience import AudienceClassifier
from classifier.topic import TopicClassifier
from classifier.industry import IndustryClassifier


MODEL_VERSION = "2026-02-08-coforge-v1"


class AppState:
    """Application state holding loaded models."""

    relevance_classifier: Optional[RelevanceClassifier] = None
    audience_classifier: Optional[AudienceClassifier] = None
    topic_classifier: Optional[TopicClassifier] = None
    industry_classifier: Optional[IndustryClassifier] = None
    startup_time: Optional[float] = None
    models_loaded: bool = False


state = AppState()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Load ML models on startup, cleanup on shutdown."""
    try:
        state.relevance_classifier = RelevanceClassifier("models/relevance.joblib")
        state.audience_classifier = AudienceClassifier("models/audience.joblib")
        state.topic_classifier = TopicClassifier("models/topic.joblib")
        state.industry_classifier = IndustryClassifier("models/industry.joblib")
        state.models_loaded = True
    except Exception:
        state.models_loaded = False
    state.startup_time = time.time()
    yield


app = FastAPI(
    title="Coforge ML Classifier",
    description="ML-based content classification for developers and entrepreneurs",
    version="1.0.0",
    lifespan=lifespan,
)

max_body_chars = 500
ms_per_second = 1000


class ClassifyRequest(BaseModel):
    """Request body for classification."""

    title: str
    body: str = ""


class ClassifyResponse(BaseModel):
    """Response body for classification."""

    relevance: str
    relevance_confidence: float
    audience: str
    audience_confidence: float
    topics: list[str]
    topic_scores: dict[str, float]
    industries: list[str]
    industry_scores: dict[str, float]
    processing_time_ms: int
    model_version: str


class HealthResponse(BaseModel):
    """Response body for health check."""

    status: str
    model_version: str
    models_loaded: bool
    uptime_seconds: float


@app.post("/classify", response_model=ClassifyResponse)
def classify(request: ClassifyRequest) -> ClassifyResponse:
    """Classify an article for coforge relevance, audience, topics, and industry."""
    start_time = time.time()

    text = f"{request.title} {request.body[:max_body_chars]}"

    if not state.models_loaded:
        processing_time_ms = int((time.time() - start_time) * ms_per_second)
        return ClassifyResponse(
            relevance="not_relevant",
            relevance_confidence=0.0,
            audience="hybrid",
            audience_confidence=0.0,
            topics=[],
            topic_scores={},
            industries=[],
            industry_scores={},
            processing_time_ms=processing_time_ms,
            model_version=MODEL_VERSION,
        )

    relevance_result = state.relevance_classifier.classify(text)
    audience_result = state.audience_classifier.classify(text)
    topic_result = state.topic_classifier.classify(text)
    industry_result = state.industry_classifier.classify(text)

    processing_time_ms = int((time.time() - start_time) * ms_per_second)

    return ClassifyResponse(
        relevance=relevance_result["relevance"],
        relevance_confidence=relevance_result["confidence"],
        audience=audience_result["audience"],
        audience_confidence=audience_result["confidence"],
        topics=topic_result["topics"],
        topic_scores=topic_result["scores"],
        industries=industry_result["industries"],
        industry_scores=industry_result["scores"],
        processing_time_ms=processing_time_ms,
        model_version=MODEL_VERSION,
    )


@app.get("/health", response_model=HealthResponse)
def health() -> HealthResponse:
    """Health check endpoint. Returns 200 only if models are loaded."""
    uptime = time.time() - state.startup_time if state.startup_time else 0
    response = HealthResponse(
        status="healthy" if state.models_loaded else "unhealthy",
        model_version=MODEL_VERSION,
        models_loaded=state.models_loaded,
        uptime_seconds=uptime,
    )
    if not state.models_loaded:
        raise HTTPException(status_code=503, detail="Models not loaded")
    return response
```

**Step 2: Commit**

```bash
git add coforge-ml/main.py
git commit -m "feat(coforge-ml): add FastAPI server with models_loaded guard"
```

---

## Task 6: Create Python Test Suite

**Files:**
- Create: `coforge-ml/tests/test_api.py`
- Create: `coforge-ml/tests/test_relevance.py`
- Create: `coforge-ml/tests/test_audience.py`
- Create: `coforge-ml/tests/test_topic.py`
- Create: `coforge-ml/tests/test_industry.py`

**Step 1: Create `coforge-ml/tests/test_api.py`**

```python
import pytest
from fastapi.testclient import TestClient
from main import app


@pytest.fixture(scope="module")
def client():
    """Create a test client with lifespan management."""
    with TestClient(app) as c:
        yield c


class TestHealthEndpoint:
    def test_health_returns_ok(self, client):
        response = client.get("/health")

        assert response.status_code == 200
        assert response.json()["status"] == "healthy"
        assert response.json()["models_loaded"] is True
        assert "model_version" in response.json()


class TestClassifyEndpoint:
    def test_classify_returns_all_fields(self, client):
        response = client.post("/classify", json={
            "title": "AI startup open-sources developer SDK",
            "body": "A new fintech company released their API toolkit."
        })

        assert response.status_code == 200
        data = response.json()

        assert "relevance" in data
        assert "relevance_confidence" in data
        assert "audience" in data
        assert "audience_confidence" in data
        assert "topics" in data
        assert "topic_scores" in data
        assert "industries" in data
        assert "industry_scores" in data
        assert "processing_time_ms" in data
        assert "model_version" in data

    def test_classify_with_empty_body(self, client):
        response = client.post("/classify", json={
            "title": "React 20 released with new features",
            "body": ""
        })

        assert response.status_code == 200

    def test_classify_with_missing_body(self, client):
        response = client.post("/classify", json={
            "title": "Series A funding announced"
        })

        assert response.status_code == 200
```

**Step 2: Create `coforge-ml/tests/test_relevance.py`**

```python
import pytest
from classifier.relevance import RelevanceClassifier


class TestRelevanceClassifier:
    @pytest.fixture(autouse=True)
    def setup(self):
        self.classifier = RelevanceClassifier("models/relevance.joblib")

    def test_classify_returns_relevance_and_confidence(self):
        result = self.classifier.classify("AI startup releases developer SDK")
        assert "relevance" in result
        assert "confidence" in result
        assert 0.0 <= result["confidence"] <= 1.0
        assert result["relevance"] in ["core_coforge", "peripheral", "not_relevant"]

    def test_empty_text_returns_not_relevant(self):
        result = self.classifier.classify("")
        assert result["relevance"] == "not_relevant"
        assert result["confidence"] == 0.5
```

**Step 3: Create `coforge-ml/tests/test_audience.py`**

```python
import pytest
from classifier.audience import AudienceClassifier


class TestAudienceClassifier:
    @pytest.fixture(autouse=True)
    def setup(self):
        self.classifier = AudienceClassifier("models/audience.joblib")

    def test_classify_returns_audience_and_confidence(self):
        result = self.classifier.classify("React framework release new hooks API")
        assert "audience" in result
        assert "confidence" in result
        assert 0.0 <= result["confidence"] <= 1.0
        assert result["audience"] in ["developer", "entrepreneur", "hybrid"]

    def test_empty_text_returns_hybrid(self):
        result = self.classifier.classify("")
        assert result["audience"] == "hybrid"
        assert result["confidence"] == 0.5
```

**Step 4: Create `coforge-ml/tests/test_topic.py`**

```python
import pytest
from classifier.topic import TopicClassifier, TOPIC_CLASSES


class TestTopicClassifier:
    @pytest.fixture(autouse=True)
    def setup(self):
        self.classifier = TopicClassifier("models/topic.joblib")

    def test_classify_returns_topics_and_scores(self):
        result = self.classifier.classify("AI startup raises Series A funding")
        assert "topics" in result
        assert "scores" in result
        assert isinstance(result["topics"], list)
        assert isinstance(result["scores"], dict)

    def test_all_scores_are_valid(self):
        result = self.classifier.classify("Open source framework release")
        for score in result["scores"].values():
            assert 0.0 <= score <= 1.0

    def test_empty_text_returns_empty(self):
        result = self.classifier.classify("")
        assert result["topics"] == []
        assert result["scores"] == {}
```

**Step 5: Create `coforge-ml/tests/test_industry.py`**

```python
import pytest
from classifier.industry import IndustryClassifier, INDUSTRY_CLASSES


class TestIndustryClassifier:
    @pytest.fixture(autouse=True)
    def setup(self):
        self.classifier = IndustryClassifier("models/industry.joblib")

    def test_classify_returns_industries_and_scores(self):
        result = self.classifier.classify("Cloud security platform raises funding")
        assert "industries" in result
        assert "scores" in result
        assert isinstance(result["industries"], list)
        assert isinstance(result["scores"], dict)

    def test_all_scores_are_valid(self):
        result = self.classifier.classify("Fintech SaaS acquisition")
        for score in result["scores"].values():
            assert 0.0 <= score <= 1.0

    def test_empty_text_returns_empty(self):
        result = self.classifier.classify("")
        assert result["industries"] == []
        assert result["scores"] == {}
```

**Step 6: Generate models then run all tests**

Run: `cd coforge-ml && python train_and_export.py && python -m pytest tests/ -v`
Expected: All tests PASS

**Step 7: Commit**

```bash
git add coforge-ml/tests/
git commit -m "test(coforge-ml): add full test suite for API, relevance, audience, topic, industry"
```

---

## Task 7: Add coforge-ml to Docker Compose

**Files:**
- Modify: `docker-compose.base.yml`
- Modify: `docker-compose.dev.yml`
- Modify: `docker-compose.prod.yml`

**Step 1: Add coforge-ml service to `docker-compose.base.yml`**

Add after the `mining-ml` service definition:

```yaml
  coforge-ml:
    build:
      context: ./coforge-ml
      dockerfile: Dockerfile
    image: docker.io/jonesrussell/coforge-ml:latest
    deploy:
      resources:
        limits:
          cpus: "0.5"
          memory: 512M
    environment:
      MODEL_PATH: /app/models
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8078/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 15s
```

**Step 2: Add coforge-ml dev overrides to `docker-compose.dev.yml`**

Add port mapping for coforge-ml:
```yaml
  coforge-ml:
    ports:
      - "${COFORGE_ML_PORT:-8078}:8078"
```

Add to classifier service `depends_on`:
```yaml
    coforge-ml:
      condition: service_healthy
```

Add to classifier service `environment`:
```yaml
    COFORGE_ENABLED: "${COFORGE_ENABLED:-false}"
    COFORGE_ML_SERVICE_URL: "http://coforge-ml:8078"
```

**Note**: Default `COFORGE_ENABLED` to `false` — opt-in until the Go integration is wired up (Task 10+).

**Step 3: Add coforge-ml prod overrides to `docker-compose.prod.yml`**

Add coforge-ml image reference and classifier dependency (same pattern as crime-ml and mining-ml in prod).

**Step 4: Commit**

```bash
git add docker-compose.base.yml docker-compose.dev.yml docker-compose.prod.yml
git commit -m "feat(docker): add coforge-ml sidecar to compose stack"
```

---

## Task 8: Add Coforge Mapping to Elasticsearch

**Files:**
- Modify: `index-manager/internal/elasticsearch/mappings/classified_content.go`
- Modify: `index-manager/internal/elasticsearch/mappings/mappings_test.go`
- Modify: `index-manager/internal/elasticsearch/mappings/versions.go`

**Step 1: Write failing test**

Add to `mappings_test.go`:

```go
func TestGetClassifiedContentMapping_NestedCoforgeFields(t *testing.T) {
	t.Helper()

	mapping := mappings.GetClassifiedContentMapping(1, 1)
	properties := mapping["mappings"].(map[string]any)["properties"].(map[string]any)

	coforgeObj, ok := properties["coforge"].(map[string]any)
	if !ok {
		t.Fatal("coforge field missing or not an object")
	}
	coforgeProps, ok := coforgeObj["properties"].(map[string]any)
	if !ok {
		t.Fatal("coforge.properties missing")
	}

	expectedCoforgeFields := []string{
		"relevance", "relevance_confidence", "audience", "audience_confidence",
		"topics", "industries", "final_confidence", "review_required", "model_version",
	}
	for _, field := range expectedCoforgeFields {
		if _, exists := coforgeProps[field]; !exists {
			t.Errorf("coforge missing field %q", field)
		}
	}
}
```

Also add `"coforge"` to the `classificationFields` list in `TestGetClassifiedContentMapping_ClassificationFields`.

**Step 2: Run test to verify it fails**

Run: `cd index-manager && go test ./internal/elasticsearch/mappings/ -v -run "Coforge"`
Expected: FAIL — coforge field missing

**Step 3: Add `getCoforgeMapping()` to `classified_content.go`**

Add after `getMiningMapping()`:

```go
// getCoforgeMapping returns the nested coforge object mapping
func getCoforgeMapping() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"relevance": map[string]any{
				"type": "keyword",
			},
			"relevance_confidence": map[string]any{
				"type": "float",
			},
			"audience": map[string]any{
				"type": "keyword",
			},
			"audience_confidence": map[string]any{
				"type": "float",
			},
			"topics": map[string]any{
				"type": "keyword",
			},
			"industries": map[string]any{
				"type": "keyword",
			},
			"final_confidence": map[string]any{
				"type": "float",
			},
			"review_required": map[string]any{
				"type": "boolean",
			},
			"model_version": map[string]any{
				"type": "keyword",
			},
		},
	}
}
```

Add to `getClassificationFields()`:
```go
"coforge": getCoforgeMapping(),
```

**Step 4: Bump classified_content mapping version**

In `versions.go`, bump `ClassifiedContentMappingVersion` from `"2.0.0"` to `"2.1.0"` (minor version — field addition).

**Step 5: Run tests to verify they pass**

Run: `cd index-manager && go test ./internal/elasticsearch/mappings/ -v`
Expected: PASS

**Step 6: Lint**

Run: `cd index-manager && golangci-lint run`
Expected: No errors

**Step 7: Commit**

```bash
git add index-manager/internal/elasticsearch/mappings/
git commit -m "feat(index-manager): add coforge nested object to classified_content mapping"
```

---

## Task 9: Add CoforgeResult to Classifier Domain

**Files:**
- Modify: `classifier/internal/domain/classification.go`

**Step 1: Add CoforgeResult struct**

Add after `MiningResult`:

```go
// CoforgeResult holds Coforge hybrid classification results.
type CoforgeResult struct {
	Relevance           string   `json:"relevance"`
	RelevanceConfidence float64  `json:"relevance_confidence"`
	Audience            string   `json:"audience"`
	AudienceConfidence  float64  `json:"audience_confidence"`
	Topics              []string `json:"topics"`
	Industries          []string `json:"industries"`
	FinalConfidence     float64  `json:"final_confidence"`
	ReviewRequired      bool     `json:"review_required"`
	ModelVersion        string   `json:"model_version,omitempty"`
}
```

**Step 2: Add Coforge field to ClassificationResult**

Add after the Mining field:
```go
// Coforge hybrid classification (optional)
Coforge *CoforgeResult `json:"coforge,omitempty"`
```

**Step 3: Add Coforge field to ClassifiedContent**

Add after the Mining field:
```go
// Coforge hybrid classification (optional)
Coforge *CoforgeResult `json:"coforge,omitempty"`
```

**Step 4: Lint**

Run: `cd classifier && golangci-lint run`
Expected: No errors

**Step 5: Commit**

```bash
git add classifier/internal/domain/classification.go
git commit -m "feat(classifier): add CoforgeResult domain type"
```

---

## Task 10: Create Go HTTP Client for coforge-ml

**Files:**
- Create: `classifier/internal/coforgemlclient/client.go`
- Create: `classifier/internal/coforgemlclient/client_test.go`

**Step 1: Write failing test `classifier/internal/coforgemlclient/client_test.go`**

Follows `miningmlclient/client_test.go` pattern exactly:

```go
//nolint:testpackage // Testing internal client requires same package access
package coforgemlclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Classify(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/classify" {
			t.Errorf("expected /classify, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		response := ClassifyResponse{
			Relevance:           "core_coforge",
			RelevanceConfidence: 0.92,
			Audience:            "hybrid",
			AudienceConfidence:  0.78,
			Topics:              []string{"funding_round", "devtools"},
			TopicScores:         map[string]float64{"funding_round": 0.88, "devtools": 0.71},
			Industries:          []string{"saas", "ai_ml"},
			IndustryScores:      map[string]float64{"saas": 0.82, "ai_ml": 0.65},
			ProcessingTimeMs:    52,
			ModelVersion:        "2026-02-08-coforge-v1",
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.Classify(context.Background(), "AI startup open-sources SDK", "Fintech company...")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Relevance != "core_coforge" {
		t.Errorf("expected core_coforge, got %s", result.Relevance)
	}
	if result.Audience != "hybrid" {
		t.Errorf("expected hybrid, got %s", result.Audience)
	}
	if len(result.Topics) != 2 {
		t.Errorf("expected 2 topics, got %d", len(result.Topics))
	}
	if len(result.Industries) != 2 {
		t.Errorf("expected 2 industries, got %d", len(result.Industries))
	}
	if result.ModelVersion != "2026-02-08-coforge-v1" {
		t.Errorf("expected model_version, got %s", result.ModelVersion)
	}
}

func TestClient_Classify_Non200(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Classify(context.Background(), "title", "body")

	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestClient_Health(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("expected /health, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.Health(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_HealthUnhealthy(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.Health(context.Background())

	if err == nil {
		t.Fatal("expected error for unhealthy service")
	}
}

func TestClient_Classify_ErrUnavailable(t *testing.T) {
	t.Helper()

	client := NewClient("http://localhost:99999")
	_, err := client.Classify(context.Background(), "title", "body")

	if err == nil {
		t.Fatal("expected error for unreachable service")
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("expected ErrUnavailable, got %v", err)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd classifier && go test ./internal/coforgemlclient/ -v`
Expected: FAIL — package doesn't exist

**Step 3: Create `classifier/internal/coforgemlclient/client.go`**

Follows `miningmlclient/client.go` pattern exactly:

```go
package coforgemlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const defaultTimeout = 5 * time.Second

// ErrUnavailable indicates the coforge ML service is unreachable.
var ErrUnavailable = errors.New("coforge ML service unavailable")

// Client is an HTTP client for the Coforge ML service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// ClassifyRequest is the request body for /classify.
type ClassifyRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// ClassifyResponse is the response body from /classify.
type ClassifyResponse struct {
	Relevance           string             `json:"relevance"`
	RelevanceConfidence float64            `json:"relevance_confidence"`
	Audience            string             `json:"audience"`
	AudienceConfidence  float64            `json:"audience_confidence"`
	Topics              []string           `json:"topics"`
	TopicScores         map[string]float64 `json:"topic_scores"`
	Industries          []string           `json:"industries"`
	IndustryScores      map[string]float64 `json:"industry_scores"`
	ProcessingTimeMs    int64              `json:"processing_time_ms"`
	ModelVersion        string             `json:"model_version"`
}

// NewClient creates a new Coforge ML client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// Classify sends a classification request to the Coforge ML service.
// Returns ErrUnavailable when the service is unreachable.
func (c *Client) Classify(ctx context.Context, title, body string) (*ClassifyResponse, error) {
	reqBody, err := json.Marshal(ClassifyRequest{Title: title, Body: body})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/classify", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coforge ML service returned %d", resp.StatusCode)
	}

	var result ClassifyResponse
	decodeErr := json.NewDecoder(resp.Body).Decode(&result)
	if decodeErr != nil {
		return nil, fmt.Errorf("decode response: %w", decodeErr)
	}

	return &result, nil
}

// Health checks if the Coforge ML service is healthy.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", http.NoBody)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("coforge ML unhealthy: %d", resp.StatusCode)
	}

	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd classifier && go test ./internal/coforgemlclient/ -v`
Expected: PASS (all 5 tests)

**Step 5: Lint**

Run: `cd classifier && golangci-lint run ./internal/coforgemlclient/`
Expected: No errors

**Step 6: Commit**

```bash
git add classifier/internal/coforgemlclient/
git commit -m "feat(classifier): add coforge-ml HTTP client"
```

---

## Task 11: Create Coforge Rules and Hybrid Classifier

**Files:**
- Create: `classifier/internal/classifier/coforge_rules.go`
- Create: `classifier/internal/classifier/coforge.go`
- Create: `classifier/internal/classifier/coforge_test.go`

**Step 1: Create `classifier/internal/classifier/coforge_rules.go`**

Follows `mining_rules.go` pattern:

```go
package classifier

import (
	"regexp"
	"strings"
)

// Coforge relevance constants.
const (
	coforgeRelevanceCore       = "core_coforge"
	coforgeRelevancePeripheral = "peripheral"
	coforgeRelevanceNot        = "not_relevant"
)

// Coforge rule confidence constants.
const (
	coforgeConfidenceCore       = 0.90
	coforgeConfidencePeripheral = 0.70
	coforgeConfidenceDefault    = 0.5
	coforgeRuleMLDisagreeWeight = 0.7
	coforgeMLOverrideThreshold  = 0.90
	coforgeBothAgreeWeight      = 2.0
	coforgeMLOverrideWeight     = 0.8
)

// coforgeRuleResult holds the result of rule-based coforge classification.
type coforgeRuleResult struct {
	relevance  string
	confidence float64
}

// Core coforge patterns - strong dev+entrepreneur intersection signal.
var coforgeCorePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(startup|company)\s+(open[- ]source|release|launch)\s+(sdk|api|tool|framework)`),
	regexp.MustCompile(`(?i)(series\s+[a-c]|seed\s+round|raised?\s+\$[\d.]+[mb])\s+.*(developer|dev\s+tool|sdk|api|platform)`),
	regexp.MustCompile(`(?i)(developer|dev)\s+(tool|platform|sdk|api)\s+.*(funding|launch|acqui)`),
	regexp.MustCompile(`(?i)(open[- ]source)\s+.*(business|revenue|funding|monetiz)`),
}

// Peripheral coforge patterns - single-domain signal.
var coforgePeripheralPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(series\s+[a-c]|seed\s+round|ipo|funding\s+round)\b`),
	regexp.MustCompile(`(?i)\b(framework|sdk|api)\s+(release|launch|update)\b`),
	regexp.MustCompile(`(?i)\b(open[- ]source|github|npm|crates\.io)\b`),
	regexp.MustCompile(`(?i)\b(acqui\w+|merger|partner\w+)\b`),
	regexp.MustCompile(`(?i)\b(saas|devtools|developer\s+experience)\b`),
}

const coforgeRuleMaxBodyChars = 500

// classifyCoforgeByRules applies rule-based coforge classification.
func classifyCoforgeByRules(title, body string) *coforgeRuleResult {
	text := strings.ToLower(title + " " + body)
	if len(body) > coforgeRuleMaxBodyChars {
		text = strings.ToLower(title + " " + body[:coforgeRuleMaxBodyChars])
	}

	for _, p := range coforgeCorePatterns {
		if p.MatchString(text) {
			return &coforgeRuleResult{relevance: coforgeRelevanceCore, confidence: coforgeConfidenceCore}
		}
	}

	for _, p := range coforgePeripheralPatterns {
		if p.MatchString(text) {
			return &coforgeRuleResult{relevance: coforgeRelevancePeripheral, confidence: coforgeConfidencePeripheral}
		}
	}

	return &coforgeRuleResult{relevance: coforgeRelevanceNot, confidence: coforgeConfidenceDefault}
}
```

**Step 2: Create `classifier/internal/classifier/coforge.go`**

Follows `mining.go` pattern:

```go
package classifier

import (
	"context"

	"github.com/jonesrussell/north-cloud/classifier/internal/coforgemlclient"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const coforgeMaxBodyChars = 500

// CoforgeMLClassifier defines the interface for Coforge ML classification.
type CoforgeMLClassifier interface {
	Classify(ctx context.Context, title, body string) (*coforgemlclient.ClassifyResponse, error)
	Health(ctx context.Context) error
}

// CoforgeClassifier implements hybrid rule + ML coforge classification.
type CoforgeClassifier struct {
	mlClient CoforgeMLClassifier
	logger   infralogger.Logger
	enabled  bool
}

// NewCoforgeClassifier creates a new hybrid coforge classifier.
func NewCoforgeClassifier(mlClient CoforgeMLClassifier, logger infralogger.Logger, enabled bool) *CoforgeClassifier {
	return &CoforgeClassifier{
		mlClient: mlClient,
		logger:   logger,
		enabled:  enabled,
	}
}

// Classify performs hybrid coforge classification on raw content.
// Returns (nil, nil) when classification is disabled.
func (s *CoforgeClassifier) Classify(ctx context.Context, raw *domain.RawContent) (*domain.CoforgeResult, error) {
	if !s.enabled {
		return nil, nil //nolint:nilnil // Intentional: nil result signals disabled
	}

	ruleResult := classifyCoforgeByRules(raw.Title, raw.RawText)

	var mlResult *coforgemlclient.ClassifyResponse
	if s.mlClient != nil {
		body := raw.RawText
		if len(body) > coforgeMaxBodyChars {
			body = body[:coforgeMaxBodyChars]
		}
		var err error
		mlResult, err = s.mlClient.Classify(ctx, raw.Title, body)
		if err != nil {
			s.logger.Warn("Coforge ML classification failed, using rules only",
				infralogger.String("content_id", raw.ID),
				infralogger.Error(err))
		}
	}

	return s.mergeResults(ruleResult, mlResult), nil
}

// mergeResults combines rule and ML results using the decision matrix.
func (s *CoforgeClassifier) mergeResults(rule *coforgeRuleResult, ml *coforgemlclient.ClassifyResponse) *domain.CoforgeResult {
	result := &domain.CoforgeResult{
		Relevance:       rule.relevance,
		FinalConfidence: rule.confidence,
	}

	if ml != nil {
		result.ModelVersion = ml.ModelVersion
		result.Audience = ml.Audience
		result.AudienceConfidence = ml.AudienceConfidence
		result.Topics = append([]string{}, ml.Topics...)
		result.Industries = append([]string{}, ml.Industries...)
	}

	s.applyDecisionLogic(result, rule, ml)

	return result
}

// applyDecisionLogic applies the decision matrix for coforge relevance.
func (s *CoforgeClassifier) applyDecisionLogic(result *domain.CoforgeResult, rule *coforgeRuleResult, ml *coforgemlclient.ClassifyResponse) {
	switch {
	case rule.relevance == coforgeRelevanceCore && ml != nil && ml.Relevance == coforgeRelevanceCore:
		result.Relevance = coforgeRelevanceCore
		result.FinalConfidence = (rule.confidence + ml.RelevanceConfidence) / coforgeBothAgreeWeight
		result.ReviewRequired = false

	case rule.relevance == coforgeRelevanceCore && ml != nil && ml.Relevance == coforgeRelevanceNot:
		result.Relevance = coforgeRelevanceCore
		result.FinalConfidence = rule.confidence * coforgeRuleMLDisagreeWeight
		result.ReviewRequired = true

	case rule.relevance == coforgeRelevanceCore:
		result.Relevance = coforgeRelevanceCore
		result.FinalConfidence = rule.confidence
		result.ReviewRequired = false

	case ml != nil && ml.Relevance == coforgeRelevanceCore && ml.RelevanceConfidence >= coforgeMLOverrideThreshold:
		result.Relevance = coforgeRelevancePeripheral
		result.FinalConfidence = ml.RelevanceConfidence * coforgeMLOverrideWeight
		result.ReviewRequired = true

	case rule.relevance == coforgeRelevancePeripheral && ml != nil && ml.Relevance == coforgeRelevanceCore:
		result.Relevance = coforgeRelevanceCore
		result.FinalConfidence = ml.RelevanceConfidence
		result.ReviewRequired = false

	default:
		result.Relevance = rule.relevance
		result.FinalConfidence = rule.confidence
	}
}
```

**Step 3: Create `classifier/internal/classifier/coforge_test.go`**

Follows `mining_test.go` pattern:

```go
//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/coforgemlclient"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

type mockCoforgeMLClient struct {
	response *coforgemlclient.ClassifyResponse
	err      error
}

func (m *mockCoforgeMLClient) Classify(_ context.Context, _, _ string) (*coforgemlclient.ClassifyResponse, error) {
	return m.response, m.err
}

func (m *mockCoforgeMLClient) Health(_ context.Context) error {
	return nil
}

func TestCoforgeClassifier_Classify_Disabled(t *testing.T) {
	t.Helper()

	cc := NewCoforgeClassifier(nil, &mockLogger{}, false)

	raw := &domain.RawContent{
		ID:    "test-1",
		Title: "AI startup releases developer SDK",
	}

	result, err := cc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when disabled")
	}
}

func TestCoforgeClassifier_Classify_RulesOnly_Peripheral(t *testing.T) {
	t.Helper()

	cc := NewCoforgeClassifier(nil, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-2",
		Title:   "Series A funding round announced",
		RawText: "The startup raised $5M.",
	}

	result, err := cc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result when rules match")
	}
	if result.Relevance != coforgeRelevancePeripheral {
		t.Errorf("expected peripheral, got %s", result.Relevance)
	}
}

func TestCoforgeClassifier_Classify_BothAgree(t *testing.T) {
	t.Helper()

	mlMock := &mockCoforgeMLClient{
		response: &coforgemlclient.ClassifyResponse{
			Relevance:           "core_coforge",
			RelevanceConfidence: 0.92,
			Audience:            "hybrid",
			AudienceConfidence:  0.78,
			Topics:              []string{"funding_round", "devtools"},
			Industries:          []string{"saas"},
			ModelVersion:        "2026-02-08-coforge-v1",
		},
	}

	cc := NewCoforgeClassifier(mlMock, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-3",
		Title:   "Startup open-sources developer SDK after Series A",
		RawText: "The company raised $10M and released their API toolkit.",
	}

	result, err := cc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Relevance != coforgeRelevanceCore {
		t.Errorf("expected core_coforge, got %s", result.Relevance)
	}
	if result.Audience != "hybrid" {
		t.Errorf("expected hybrid, got %s", result.Audience)
	}
	if len(result.Topics) != 2 {
		t.Errorf("expected 2 topics, got %d", len(result.Topics))
	}
}

func TestCoforgeClassifier_Classify_RulesOnly_NotRelevant(t *testing.T) {
	t.Helper()

	cc := NewCoforgeClassifier(nil, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-4",
		Title:   "Weather forecast for the weekend",
		RawText: "Sunny skies expected.",
	}

	result, err := cc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Relevance != coforgeRelevanceNot {
		t.Errorf("expected not_relevant, got %s", result.Relevance)
	}
}
```

**Step 4: Run tests**

Run: `cd classifier && go test ./internal/classifier/ -v -run "Coforge"`
Expected: PASS (all 4 tests)

**Step 5: Lint**

Run: `cd classifier && golangci-lint run`
Expected: No errors

**Step 6: Commit**

```bash
git add classifier/internal/classifier/coforge_rules.go classifier/internal/classifier/coforge.go classifier/internal/classifier/coforge_test.go
git commit -m "feat(classifier): add coforge hybrid rules + ML classifier"
```

---

## Task 12: Wire Coforge Classifier into Pipeline

**Files:**
- Modify: `classifier/internal/config/config.go`
- Modify: `classifier/internal/classifier/classifier.go`
- Modify: `classifier/internal/bootstrap/classifier.go`

**Step 1: Add CoforgeConfig to config.go**

Add to the defaults section at top of file:
```go
defaultCoforgeMLServiceURL = "http://coforge-ml:8078"
```

Add after `MiningConfig`:
```go
// CoforgeConfig holds Coforge hybrid classification settings.
type CoforgeConfig struct {
	Enabled      bool   `env:"COFORGE_ENABLED"        yaml:"enabled"`
	MLServiceURL string `env:"COFORGE_ML_SERVICE_URL" yaml:"ml_service_url"`
}
```

Add to `ClassificationConfig`:
```go
Coforge CoforgeConfig `yaml:"coforge"`
```

Add to `setClassificationDefaults()`:
```go
if c.Coforge.MLServiceURL == "" {
	c.Coforge.MLServiceURL = defaultCoforgeMLServiceURL
}
```

**Step 2: Add Coforge to Classifier struct and Config**

In `classifier.go`, add `coforge *CoforgeClassifier` field to the `Classifier` struct.

Add `CoforgeClassifier *CoforgeClassifier` to the `Config` struct.

In `NewClassifier()`, add: `coforge: config.CoforgeClassifier,`

**Step 3: Wire into `runOptionalClassifiers`**

Add after the mining block in `runOptionalClassifiers()`:

```go
var coforgeResult *domain.CoforgeResult
if c.coforge != nil {
	cfResult, cfErr := c.coforge.Classify(ctx, raw)
	if cfErr != nil {
		c.logger.Warn("Coforge classification failed",
			infralogger.String("content_id", raw.ID),
			infralogger.Error(cfErr))
	} else if cfResult != nil {
		coforgeResult = cfResult
	}
}
```

Update the return signature to include `*domain.CoforgeResult` and update the caller in `Classify()`.

Add `Coforge: coforgeResult,` to the `ClassificationResult` construction.

Add `Coforge: result.Coforge,` to `BuildClassifiedContent()`.

**Step 4: Add `createCoforgeClassifier` to bootstrap**

In `bootstrap/classifier.go`, add:

```go
// createCoforgeClassifier creates a Coforge classifier if enabled in config.
func createCoforgeClassifier(cfg *config.Config, logger infralogger.Logger) *classifier.CoforgeClassifier {
	if !cfg.Classification.Coforge.Enabled {
		return nil
	}

	var mlClient classifier.CoforgeMLClassifier
	if cfg.Classification.Coforge.MLServiceURL != "" {
		mlClient = coforgemlclient.NewClient(cfg.Classification.Coforge.MLServiceURL)
	}

	logger.Info("Coforge classifier enabled",
		infralogger.String("ml_service_url", cfg.Classification.Coforge.MLServiceURL))

	return classifier.NewCoforgeClassifier(mlClient, logger, true)
}
```

Add import for `coforgemlclient`.

Wire into `createClassifierConfig()`:
```go
CoforgeClassifier: createCoforgeClassifier(cfg, logger),
```

**Step 5: Run all classifier tests**

Run: `cd classifier && go test ./... -v`
Expected: PASS

**Step 6: Lint**

Run: `cd classifier && golangci-lint run`
Expected: No errors

**Step 7: Commit**

```bash
git add classifier/internal/config/config.go classifier/internal/classifier/classifier.go classifier/internal/bootstrap/classifier.go
git commit -m "feat(classifier): wire coforge classifier into classification pipeline"
```

---

## Task 13: Final Validation

Run all linters and tests across all affected services.

**Step 1: Run all tests**

```bash
cd index-manager && go test ./... -v
cd classifier && go test ./... -v
cd coforge-ml && python -m pytest tests/ -v
```

**Step 2: Run all linters**

```bash
cd index-manager && golangci-lint run
cd classifier && golangci-lint run
```

**Step 3: Build coforge-ml Docker image**

```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml build coforge-ml
```

**Step 4: Start and verify health**

```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d coforge-ml
curl http://localhost:8078/health
```

Expected: `{"status": "healthy", "model_version": "2026-02-08-coforge-v1", "models_loaded": true, ...}`

**Step 5: Test classification endpoint**

```bash
curl -X POST http://localhost:8078/classify \
  -H "Content-Type: application/json" \
  -d '{"title": "AI startup open-sources developer SDK after Series A", "body": "The fintech company raised $10M..."}'
```

Expected: Response with all 4 classifier outputs (relevance, audience, topics, industries).

---

## Summary of Changes by Service

| Service | Changes | Files |
|---------|---------|-------|
| **coforge-ml** (new) | Full ML sidecar — 4 models, FastAPI, tests | 15 files |
| **classifier** | New client, domain type, hybrid classifier, config, bootstrap | 8 files |
| **index-manager** | Coforge mapping, version bump | 3 files |
| **docker-compose** | Add coforge-ml to base, dev, prod | 3 files |
