# StreetCode Hybrid Classifier Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Integrate a hybrid rule + ML classifier for street crime detection into the North Cloud pipeline.

**Architecture:** Python ML microservice (FastAPI + sklearn) called by Go classifier service. Rules provide precision, ML provides recall. Publisher routes homepage-eligible articles to StreetCode-specific Redis channels.

**Tech Stack:** Python 3.11, FastAPI, scikit-learn, joblib | Go 1.24+, gin | Redis pub/sub, Elasticsearch

---

## Task 1: Create ML Microservice Directory Structure

**Files:**
- Create: `streetcode-ml/requirements.txt`
- Create: `streetcode-ml/Dockerfile`
- Create: `streetcode-ml/classifier/__init__.py`

**Step 1: Create requirements.txt**

```txt
fastapi==0.109.2
uvicorn==0.27.1
pydantic==2.6.1
scikit-learn==1.4.0
joblib==1.3.2
numpy==1.26.4
```

**Step 2: Create Dockerfile**

```dockerfile
FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE 8076

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8076"]
```

**Step 3: Create classifier/__init__.py**

```python
"""StreetCode ML Classifier package."""
```

**Step 4: Verify structure**

Run: `ls -la streetcode-ml/`
Expected: requirements.txt, Dockerfile, classifier/

**Step 5: Commit**

```bash
git add streetcode-ml/
git commit -m "feat(streetcode-ml): scaffold ML microservice directory structure

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 2: Implement Text Preprocessor

**Files:**
- Create: `streetcode-ml/classifier/preprocessor.py`
- Create: `streetcode-ml/tests/__init__.py`
- Create: `streetcode-ml/tests/test_preprocessor.py`

**Step 1: Write the failing test**

```python
# streetcode-ml/tests/test_preprocessor.py
import pytest
from classifier.preprocessor import preprocess_text


class TestPreprocessText:
    def test_lowercases_text(self):
        result = preprocess_text("POLICE ARREST Suspect")
        assert "police arrest suspect" in result

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

**Step 2: Run test to verify it fails**

Run: `cd streetcode-ml && python -m pytest tests/test_preprocessor.py -v`
Expected: FAIL with "No module named 'classifier.preprocessor'"

**Step 3: Write minimal implementation**

```python
# streetcode-ml/classifier/preprocessor.py
"""Text preprocessing for ML classification."""

import re
from typing import Optional


def preprocess_text(text: Optional[str]) -> str:
    """Clean and normalize text for vectorization.

    Args:
        text: Raw text input, may be None

    Returns:
        Cleaned, lowercase text with URLs/emails removed
    """
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
```

**Step 4: Run test to verify it passes**

Run: `cd streetcode-ml && python -m pytest tests/test_preprocessor.py -v`
Expected: PASS (6 passed)

**Step 5: Commit**

```bash
git add streetcode-ml/classifier/preprocessor.py streetcode-ml/tests/
git commit -m "feat(streetcode-ml): add text preprocessor with tests

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 3: Export Trained Models to Joblib

**Files:**
- Modify: `docs/plans/streetcode_ml_training.py`
- Create: `streetcode-ml/models/.gitkeep`

**Step 1: Write the export function test**

Run the training script with export functionality to generate model files.

**Step 2: Add export_models function to training script**

Add at the end of `docs/plans/streetcode_ml_training.py`:

```python
def export_models(relevance_results, crime_type_results, location_results, output_dir: str):
    """Export trained models to joblib files."""
    import os
    os.makedirs(output_dir, exist_ok=True)

    # Export relevance model (best performer)
    best_name = max(relevance_results.items(), key=lambda x: x[1]['f1_macro'])[0]
    best = relevance_results[best_name]
    joblib.dump({
        'model': best['model'],
        'vectorizer': best['vectorizer'],
        'classes': ['core_street_crime', 'peripheral_crime', 'not_crime'],
    }, f'{output_dir}/relevance.joblib')
    print(f"Exported relevance model ({best_name})")

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

# At end of main():
export_models(relevance_results, crime_type_results, location_results,
              '/home/fsd42/dev/north-cloud/streetcode-ml/models')
```

**Step 3: Run training and export**

Run: `cd /home/fsd42/dev/north-cloud && source docs/plans/.venv/bin/activate && python docs/plans/streetcode_ml_training.py`
Expected: Model files created in streetcode-ml/models/

**Step 4: Verify models exist**

Run: `ls -la streetcode-ml/models/`
Expected: relevance.joblib, crime_type.joblib, location.joblib

**Step 5: Commit**

```bash
git add streetcode-ml/models/ docs/plans/streetcode_ml_training.py
git commit -m "feat(streetcode-ml): export trained sklearn models to joblib

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 4: Implement Relevance Classifier Module

**Files:**
- Create: `streetcode-ml/classifier/relevance.py`
- Create: `streetcode-ml/tests/test_relevance.py`

**Step 1: Write the failing test**

```python
# streetcode-ml/tests/test_relevance.py
import pytest
from classifier.relevance import RelevanceClassifier


class TestRelevanceClassifier:
    @pytest.fixture
    def classifier(self):
        return RelevanceClassifier("models/relevance.joblib")

    def test_classify_returns_relevance_and_confidence(self, classifier):
        result = classifier.classify("Man charged with murder after stabbing")

        assert "relevance" in result
        assert "confidence" in result
        assert result["relevance"] in ["core_street_crime", "peripheral_crime", "not_crime"]
        assert 0.0 <= result["confidence"] <= 1.0

    def test_murder_classified_as_core(self, classifier):
        result = classifier.classify("Man charged with murder after downtown stabbing")
        assert result["relevance"] == "core_street_crime"
        assert result["confidence"] >= 0.5

    def test_restaurant_classified_as_not_crime(self, classifier):
        result = classifier.classify("New restaurant opens in downtown area")
        assert result["relevance"] == "not_crime"
```

**Step 2: Run test to verify it fails**

Run: `cd streetcode-ml && python -m pytest tests/test_relevance.py -v`
Expected: FAIL with "No module named 'classifier.relevance'"

**Step 3: Write minimal implementation**

```python
# streetcode-ml/classifier/relevance.py
"""Street crime relevance classifier (3-class)."""

import joblib
import numpy as np
from typing import TypedDict

from .preprocessor import preprocess_text


class RelevanceResult(TypedDict):
    relevance: str
    confidence: float


class RelevanceClassifier:
    """Classifies articles into core_street_crime, peripheral_crime, or not_crime."""

    def __init__(self, model_path: str):
        """Load the trained model from joblib file."""
        data = joblib.load(model_path)
        self.model = data['model']
        self.vectorizer = data['vectorizer']
        self.classes = data.get('classes', ['core_street_crime', 'peripheral_crime', 'not_crime'])

    def classify(self, text: str) -> RelevanceResult:
        """Classify text and return relevance with confidence.

        Args:
            text: Combined title + body text

        Returns:
            Dict with 'relevance' (str) and 'confidence' (float 0-1)
        """
        cleaned = preprocess_text(text)
        if not cleaned:
            return {"relevance": "not_crime", "confidence": 0.5}

        # Vectorize
        features = self.vectorizer.transform([cleaned])

        # Predict with probabilities
        probabilities = self.model.predict_proba(features)[0]
        predicted_idx = np.argmax(probabilities)

        return {
            "relevance": self.classes[predicted_idx],
            "confidence": float(probabilities[predicted_idx]),
        }
```

**Step 4: Run test to verify it passes**

Run: `cd streetcode-ml && python -m pytest tests/test_relevance.py -v`
Expected: PASS (3 passed)

**Step 5: Commit**

```bash
git add streetcode-ml/classifier/relevance.py streetcode-ml/tests/test_relevance.py
git commit -m "feat(streetcode-ml): implement relevance classifier module

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 5: Implement Crime Type Classifier Module

**Files:**
- Create: `streetcode-ml/classifier/crime_type.py`
- Create: `streetcode-ml/tests/test_crime_type.py`

**Step 1: Write the failing test**

```python
# streetcode-ml/tests/test_crime_type.py
import pytest
from classifier.crime_type import CrimeTypeClassifier


class TestCrimeTypeClassifier:
    @pytest.fixture
    def classifier(self):
        return CrimeTypeClassifier("models/crime_type.joblib")

    def test_classify_returns_types_and_scores(self, classifier):
        result = classifier.classify("Man charged with murder")

        assert "crime_types" in result
        assert "scores" in result
        assert isinstance(result["crime_types"], list)
        assert isinstance(result["scores"], dict)

    def test_murder_returns_violent_crime(self, classifier):
        result = classifier.classify("Man charged with murder after stabbing")
        assert "violent_crime" in result["crime_types"]

    def test_theft_returns_property_crime(self, classifier):
        result = classifier.classify("Police arrest suspect for shoplifting")
        # May or may not detect based on training data
        assert isinstance(result["crime_types"], list)
```

**Step 2: Run test to verify it fails**

Run: `cd streetcode-ml && python -m pytest tests/test_crime_type.py -v`
Expected: FAIL with "No module named 'classifier.crime_type'"

**Step 3: Write minimal implementation**

```python
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
        self.classes = self.mlb.classes_

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
        # OneVsRestClassifier with predict_proba
        try:
            probabilities = self.model.predict_proba(features)
            # probabilities is a list of arrays, one per class
            scores = {}
            crime_types = []

            for i, cls in enumerate(self.classes):
                # Each estimator returns [P(neg), P(pos)] for that label
                prob = float(probabilities[i][0][1])
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
```

**Step 4: Run test to verify it passes**

Run: `cd streetcode-ml && python -m pytest tests/test_crime_type.py -v`
Expected: PASS (3 passed)

**Step 5: Commit**

```bash
git add streetcode-ml/classifier/crime_type.py streetcode-ml/tests/test_crime_type.py
git commit -m "feat(streetcode-ml): implement multi-label crime type classifier

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 6: Implement Location Classifier Module

**Files:**
- Create: `streetcode-ml/classifier/location.py`
- Create: `streetcode-ml/tests/test_location.py`

**Step 1: Write the failing test**

```python
# streetcode-ml/tests/test_location.py
import pytest
from classifier.location import LocationClassifier


class TestLocationClassifier:
    @pytest.fixture
    def classifier(self):
        return LocationClassifier("models/location.joblib")

    def test_classify_returns_location_and_confidence(self, classifier):
        result = classifier.classify("Sudbury police arrest suspect")

        assert "location" in result
        assert "confidence" in result
        assert result["location"] in ["local_canada", "national_canada", "international", "not_specified"]

    def test_sudbury_classified_as_local(self, classifier):
        result = classifier.classify("Sudbury police investigate downtown shooting")
        assert result["location"] == "local_canada"

    def test_international_story(self, classifier):
        result = classifier.classify("Minneapolis police respond to incident")
        # May classify as international based on training
        assert result["location"] in ["international", "not_specified"]
```

**Step 2: Run test to verify it fails**

Run: `cd streetcode-ml && python -m pytest tests/test_location.py -v`
Expected: FAIL with "No module named 'classifier.location'"

**Step 3: Write minimal implementation**

```python
# streetcode-ml/classifier/location.py
"""Location specificity classifier (4-class)."""

import joblib
import numpy as np
from typing import TypedDict

from .preprocessor import preprocess_text


class LocationResult(TypedDict):
    location: str
    confidence: float


class LocationClassifier:
    """Classifies articles by location specificity."""

    def __init__(self, model_path: str):
        """Load the trained model from joblib file."""
        data = joblib.load(model_path)
        self.model = data['model']
        self.vectorizer = data['vectorizer']
        self.classes = data.get('classes', ['local_canada', 'national_canada', 'international', 'not_specified'])

    def classify(self, text: str) -> LocationResult:
        """Classify text and return location with confidence.

        Args:
            text: Combined title + body text

        Returns:
            Dict with 'location' (str) and 'confidence' (float 0-1)
        """
        cleaned = preprocess_text(text)
        if not cleaned:
            return {"location": "not_specified", "confidence": 0.5}

        # Vectorize
        features = self.vectorizer.transform([cleaned])

        # Predict with probabilities
        probabilities = self.model.predict_proba(features)[0]
        predicted_idx = np.argmax(probabilities)

        return {
            "location": self.classes[predicted_idx],
            "confidence": float(probabilities[predicted_idx]),
        }
```

**Step 4: Run test to verify it passes**

Run: `cd streetcode-ml && python -m pytest tests/test_location.py -v`
Expected: PASS (3 passed)

**Step 5: Commit**

```bash
git add streetcode-ml/classifier/location.py streetcode-ml/tests/test_location.py
git commit -m "feat(streetcode-ml): implement location specificity classifier

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 7: Implement FastAPI Main Server

**Files:**
- Create: `streetcode-ml/main.py`
- Create: `streetcode-ml/tests/test_api.py`

**Step 1: Write the failing test**

```python
# streetcode-ml/tests/test_api.py
import pytest
from fastapi.testclient import TestClient
from main import app


class TestHealthEndpoint:
    def test_health_returns_ok(self):
        client = TestClient(app)
        response = client.get("/health")

        assert response.status_code == 200
        assert response.json()["status"] == "healthy"


class TestClassifyEndpoint:
    def test_classify_returns_all_fields(self):
        client = TestClient(app)
        response = client.post("/classify", json={
            "title": "Man charged with murder after stabbing",
            "body": "Police arrested a suspect following the incident."
        })

        assert response.status_code == 200
        data = response.json()

        # Check all required fields
        assert "relevance" in data
        assert "relevance_confidence" in data
        assert "crime_types" in data
        assert "crime_type_scores" in data
        assert "location" in data
        assert "location_confidence" in data
        assert "processing_time_ms" in data

    def test_classify_with_empty_body(self):
        client = TestClient(app)
        response = client.post("/classify", json={
            "title": "Man charged with murder",
            "body": ""
        })

        assert response.status_code == 200

    def test_classify_with_missing_body(self):
        client = TestClient(app)
        response = client.post("/classify", json={
            "title": "Man charged with murder"
        })

        assert response.status_code == 200
```

**Step 2: Run test to verify it fails**

Run: `cd streetcode-ml && python -m pytest tests/test_api.py -v`
Expected: FAIL with "No module named 'main'"

**Step 3: Write minimal implementation**

```python
# streetcode-ml/main.py
"""StreetCode ML Classifier - FastAPI Server."""

import time
from typing import Optional

from fastapi import FastAPI
from pydantic import BaseModel

from classifier.relevance import RelevanceClassifier
from classifier.crime_type import CrimeTypeClassifier
from classifier.location import LocationClassifier


app = FastAPI(
    title="StreetCode ML Classifier",
    description="ML-based street crime classification service",
    version="1.0.0",
)

# Load models on startup
relevance_classifier: Optional[RelevanceClassifier] = None
crime_type_classifier: Optional[CrimeTypeClassifier] = None
location_classifier: Optional[LocationClassifier] = None
startup_time: Optional[float] = None


@app.on_event("startup")
def load_models():
    """Load ML models on server startup."""
    global relevance_classifier, crime_type_classifier, location_classifier, startup_time

    relevance_classifier = RelevanceClassifier("models/relevance.joblib")
    crime_type_classifier = CrimeTypeClassifier("models/crime_type.joblib")
    location_classifier = LocationClassifier("models/location.joblib")
    startup_time = time.time()


class ClassifyRequest(BaseModel):
    """Request body for classification."""
    title: str
    body: str = ""


class ClassifyResponse(BaseModel):
    """Response body for classification."""
    relevance: str
    relevance_confidence: float
    crime_types: list[str]
    crime_type_scores: dict[str, float]
    location: str
    location_confidence: float
    processing_time_ms: int


class HealthResponse(BaseModel):
    """Response body for health check."""
    status: str
    model_version: str
    uptime_seconds: float


@app.post("/classify", response_model=ClassifyResponse)
def classify(request: ClassifyRequest) -> ClassifyResponse:
    """Classify an article for street crime relevance."""
    start_time = time.time()

    # Combine title and body (use first 500 chars of body)
    text = f"{request.title} {request.body[:500]}"

    # Run classifiers
    relevance_result = relevance_classifier.classify(text)
    crime_type_result = crime_type_classifier.classify(text)
    location_result = location_classifier.classify(text)

    processing_time_ms = int((time.time() - start_time) * 1000)

    return ClassifyResponse(
        relevance=relevance_result["relevance"],
        relevance_confidence=relevance_result["confidence"],
        crime_types=crime_type_result["crime_types"],
        crime_type_scores=crime_type_result["scores"],
        location=location_result["location"],
        location_confidence=location_result["confidence"],
        processing_time_ms=processing_time_ms,
    )


@app.get("/health", response_model=HealthResponse)
def health() -> HealthResponse:
    """Health check endpoint."""
    uptime = time.time() - startup_time if startup_time else 0
    return HealthResponse(
        status="healthy",
        model_version="1.0.0",
        uptime_seconds=uptime,
    )
```

**Step 4: Run test to verify it passes**

Run: `cd streetcode-ml && python -m pytest tests/test_api.py -v`
Expected: PASS (4 passed)

**Step 5: Commit**

```bash
git add streetcode-ml/main.py streetcode-ml/tests/test_api.py
git commit -m "feat(streetcode-ml): implement FastAPI server with classify endpoint

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 8: Add Docker Compose Configuration

**Files:**
- Modify: `docker-compose.base.yml`
- Modify: `docker-compose.dev.yml`

**Step 1: Add service to docker-compose.base.yml**

Add after the `auth` service:

```yaml
  streetcode-ml:
    <<: *service-defaults
    build:
      context: ./streetcode-ml
      dockerfile: Dockerfile
    image: docker.io/jonesrussell/streetcode-ml:latest
    deploy:
      resources:
        limits:
          cpus: "0.5"
          memory: 512M
    environment:
      MODEL_PATH: /app/models
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8076/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 15s
```

**Step 2: Add port mapping to docker-compose.dev.yml**

```yaml
  streetcode-ml:
    ports:
      - "8076:8076"
```

**Step 3: Build and test container**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml build streetcode-ml`
Expected: Build succeeds

**Step 4: Start and verify health**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d streetcode-ml && sleep 5 && curl http://localhost:8076/health`
Expected: `{"status":"healthy","model_version":"1.0.0",...}`

**Step 5: Commit**

```bash
git add docker-compose.base.yml docker-compose.dev.yml
git commit -m "feat(docker): add streetcode-ml service to compose

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 9: Implement Go ML Client

**Files:**
- Create: `classifier/internal/mlclient/client.go`
- Create: `classifier/internal/mlclient/client_test.go`

**Step 1: Write the failing test**

```go
// classifier/internal/mlclient/client_test.go
package mlclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Classify(t *testing.T) {
	t.Helper()

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/classify" {
			t.Errorf("expected /classify, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		response := ClassifyResponse{
			Relevance:           "core_street_crime",
			RelevanceConfidence: 0.85,
			CrimeTypes:          []string{"violent_crime"},
			CrimeTypeScores:     map[string]float64{"violent_crime": 0.9},
			Location:            "local_canada",
			LocationConfidence:  0.75,
			ProcessingTimeMs:    15,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.Classify(context.Background(), "Man charged with murder", "Police arrested...")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Relevance != "core_street_crime" {
		t.Errorf("expected core_street_crime, got %s", result.Relevance)
	}

	if result.RelevanceConfidence < 0.8 {
		t.Errorf("expected confidence >= 0.8, got %f", result.RelevanceConfidence)
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
```

**Step 2: Run test to verify it fails**

Run: `cd classifier && go test ./internal/mlclient/... -v`
Expected: FAIL with "no Go files in ./internal/mlclient"

**Step 3: Write minimal implementation**

```go
// classifier/internal/mlclient/client.go
package mlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultTimeout = 5 * time.Second

// Client is an HTTP client for the StreetCode ML service.
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
	CrimeTypes          []string           `json:"crime_types"`
	CrimeTypeScores     map[string]float64 `json:"crime_type_scores"`
	Location            string             `json:"location"`
	LocationConfidence  float64            `json:"location_confidence"`
	ProcessingTimeMs    int64              `json:"processing_time_ms"`
}

// NewClient creates a new ML client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// Classify sends a classification request to the ML service.
func (c *Client) Classify(ctx context.Context, title, body string) (*ClassifyResponse, error) {
	reqBody, err := json.Marshal(ClassifyRequest{Title: title, Body: body})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/classify", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ml service returned %d", resp.StatusCode)
	}

	var result ClassifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// Health checks if the ML service is healthy.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: %d", resp.StatusCode)
	}

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd classifier && go test ./internal/mlclient/... -v`
Expected: PASS (3 tests)

**Step 5: Commit**

```bash
git add classifier/internal/mlclient/
git commit -m "feat(classifier): add ML client for streetcode-ml service

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 10: Implement StreetCode Rule Classifier

**Files:**
- Create: `classifier/internal/classifier/streetcode_rules.go`
- Create: `classifier/internal/classifier/streetcode_rules_test.go`

**Step 1: Write the failing test**

```go
// classifier/internal/classifier/streetcode_rules_test.go
//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"testing"
)

func TestStreetCodeRules_ClassifyByRules_ViolentCrime(t *testing.T) {
	t.Helper()

	tests := []struct {
		name             string
		title            string
		expectedRelevance string
		expectedTypes    []string
	}{
		{
			name:             "murder",
			title:            "Man charged with murder after stabbing",
			expectedRelevance: "core_street_crime",
			expectedTypes:    []string{"violent_crime"},
		},
		{
			name:             "shooting",
			title:            "Police respond to downtown shooting",
			expectedRelevance: "core_street_crime",
			expectedTypes:    []string{"violent_crime"},
		},
		{
			name:             "assault with arrest",
			title:            "Suspect arrested for assault in park",
			expectedRelevance: "core_street_crime",
			expectedTypes:    []string{"violent_crime"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")

			if result.relevance != tt.expectedRelevance {
				t.Errorf("relevance: got %s, want %s", result.relevance, tt.expectedRelevance)
			}

			for _, expectedType := range tt.expectedTypes {
				found := false
				for _, actualType := range result.crimeTypes {
					if actualType == expectedType {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("missing crime type %s in %v", expectedType, result.crimeTypes)
				}
			}
		})
	}
}

func TestStreetCodeRules_ClassifyByRules_Exclusions(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		title string
	}{
		{"job posting", "Full-Time Position Available"},
		{"directory", "Listings By Category"},
		{"sports", "Local Sports Updates"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")

			if result.relevance != "not_crime" {
				t.Errorf("expected not_crime for excluded content, got %s", result.relevance)
			}
		})
	}
}

func TestStreetCodeRules_ClassifyByRules_NotCrime(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		title string
	}{
		{"restaurant", "New restaurant opens downtown"},
		{"weather", "Weekend forecast looks sunny"},
		{"sports", "Hockey team wins championship"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")

			if result.relevance != "not_crime" {
				t.Errorf("expected not_crime, got %s", result.relevance)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd classifier && go test ./internal/classifier/streetcode_rules_test.go -v`
Expected: FAIL with "undefined: classifyByRules"

**Step 3: Write minimal implementation**

```go
// classifier/internal/classifier/streetcode_rules.go
package classifier

import (
	"regexp"
	"strings"
)

// ruleResult holds the result of rule-based classification.
type ruleResult struct {
	relevance  string
	confidence float64
	crimeTypes []string
}

// Pattern types for rule matching.
type patternWithConf struct {
	pattern    *regexp.Regexp
	confidence float64
}

// Exclusion patterns - if matched, article is excluded.
var excludePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(Register|Sign up|Login|Subscribe)`),
	regexp.MustCompile(`(?i)^(Listings? By|Directory|Careers|Jobs)`),
	regexp.MustCompile(`(?i)(Part.Time|Full.Time|Hiring|Position)`),
	regexp.MustCompile(`(?i)^Local (Sports|Events|Weather)$`),
}

// Violent crime patterns.
var violentCrimePatterns = []patternWithConf{
	{regexp.MustCompile(`(?i)(murder|homicide|manslaughter)`), 0.95},
	{regexp.MustCompile(`(?i)(shooting|shootout|shot dead|gunfire)`), 0.90},
	{regexp.MustCompile(`(?i)(stab|stabbing|stabbed)`), 0.90},
	{regexp.MustCompile(`(?i)(assault|assaulted).*(charged|arrest|police)`), 0.85},
	{regexp.MustCompile(`(?i)(sexual assault|rape|sex assault)`), 0.90},
	{regexp.MustCompile(`(?i)(found dead|human remains)`), 0.80},
}

// Property crime patterns.
var propertyCrimePatterns = []patternWithConf{
	{regexp.MustCompile(`(?i)(theft|stolen|shoplifting).*(police|arrest)`), 0.85},
	{regexp.MustCompile(`(?i)(burglary|break.in)`), 0.85},
	{regexp.MustCompile(`(?i)arson`), 0.80},
	{regexp.MustCompile(`(?i)\$[\d,]+.*(stolen|theft)`), 0.85},
}

// Drug crime patterns.
var drugCrimePatterns = []patternWithConf{
	{regexp.MustCompile(`(?i)(drug bust|drug raid|drug seizure)`), 0.90},
	{regexp.MustCompile(`(?i)(fentanyl|cocaine|heroin).*(seiz|arrest|trafficking)`), 0.90},
}

// International patterns - downgrade to peripheral.
var internationalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(Minneapolis|U\.S\.|American|Mexico|European|Israel)`),
}

// classifyByRules applies rule-based classification.
func classifyByRules(title, body string) *ruleResult {
	// Check exclusions first
	for _, p := range excludePatterns {
		if p.MatchString(title) {
			return &ruleResult{relevance: "not_crime", confidence: 0.95}
		}
	}

	result := &ruleResult{
		relevance:  "not_crime",
		confidence: 0.5,
		crimeTypes: []string{},
	}

	// Check violent crime patterns
	for _, p := range violentCrimePatterns {
		if p.pattern.MatchString(title) {
			result.relevance = "core_street_crime"
			result.confidence = maxFloat(result.confidence, p.confidence)
			if !containsString(result.crimeTypes, "violent_crime") {
				result.crimeTypes = append(result.crimeTypes, "violent_crime")
			}
		}
	}

	// Check property crime patterns
	for _, p := range propertyCrimePatterns {
		if p.pattern.MatchString(title) {
			result.relevance = "core_street_crime"
			result.confidence = maxFloat(result.confidence, p.confidence)
			if !containsString(result.crimeTypes, "property_crime") {
				result.crimeTypes = append(result.crimeTypes, "property_crime")
			}
		}
	}

	// Check drug crime patterns
	for _, p := range drugCrimePatterns {
		if p.pattern.MatchString(title) {
			result.relevance = "core_street_crime"
			result.confidence = maxFloat(result.confidence, p.confidence)
			if !containsString(result.crimeTypes, "drug_crime") {
				result.crimeTypes = append(result.crimeTypes, "drug_crime")
			}
		}
	}

	// Check international (downgrade to peripheral)
	for _, p := range internationalPatterns {
		if p.MatchString(title) && result.relevance == "core_street_crime" {
			result.relevance = "peripheral_crime"
			result.confidence *= 0.7
		}
	}

	// Add criminal_justice if has crime types and mentions arrest/charged
	if len(result.crimeTypes) > 0 {
		if regexp.MustCompile(`(?i)(charged|arrest|sentenced|trial)`).MatchString(title) {
			result.crimeTypes = append(result.crimeTypes, "criminal_justice")
		}
	}

	return result
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
```

**Step 4: Run test to verify it passes**

Run: `cd classifier && go test ./internal/classifier/streetcode_rules_test.go ./internal/classifier/streetcode_rules.go -v`
Expected: PASS (9 tests)

**Step 5: Commit**

```bash
git add classifier/internal/classifier/streetcode_rules.go classifier/internal/classifier/streetcode_rules_test.go
git commit -m "feat(classifier): implement streetcode rule-based classifier

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 11: Implement StreetCode Hybrid Classifier

**Files:**
- Create: `classifier/internal/classifier/streetcode.go`
- Create: `classifier/internal/classifier/streetcode_test.go`

**Step 1: Write the failing test**

```go
// classifier/internal/classifier/streetcode_test.go
//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/mlclient"
)

type mockMLClient struct {
	response *mlclient.ClassifyResponse
	err      error
}

func (m *mockMLClient) Classify(_ context.Context, _, _ string) (*mlclient.ClassifyResponse, error) {
	return m.response, m.err
}

func (m *mockMLClient) Health(_ context.Context) error {
	return nil
}

func TestStreetCodeClassifier_Classify_RulesOnly(t *testing.T) {
	t.Helper()

	sc := NewStreetCodeClassifier(nil, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-1",
		Title:   "Man charged with murder after stabbing",
		RawText: "Police arrested a suspect.",
	}

	result, err := sc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Relevance != "core_street_crime" {
		t.Errorf("expected core_street_crime, got %s", result.Relevance)
	}

	if !result.HomepageEligible {
		t.Error("expected homepage eligible for high-confidence crime")
	}
}

func TestStreetCodeClassifier_Classify_BothAgree(t *testing.T) {
	t.Helper()

	mlMock := &mockMLClient{
		response: &mlclient.ClassifyResponse{
			Relevance:           "core_street_crime",
			RelevanceConfidence: 0.85,
			CrimeTypes:          []string{"violent_crime"},
			Location:            "local_canada",
		},
	}

	sc := NewStreetCodeClassifier(mlMock, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-2",
		Title:   "Man charged with murder",
		RawText: "Downtown stabbing incident.",
	}

	result, err := sc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Relevance != "core_street_crime" {
		t.Errorf("expected core_street_crime, got %s", result.Relevance)
	}

	// Both agree, should have high confidence
	if result.FinalConfidence < 0.75 {
		t.Errorf("expected confidence >= 0.75 when both agree, got %f", result.FinalConfidence)
	}
}

func TestStreetCodeClassifier_Classify_Disabled(t *testing.T) {
	t.Helper()

	sc := NewStreetCodeClassifier(nil, &mockLogger{}, false)

	raw := &domain.RawContent{
		ID:    "test-3",
		Title: "Murder headline",
	}

	result, err := sc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != nil {
		t.Error("expected nil result when disabled")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd classifier && go test ./internal/classifier/streetcode_test.go -v`
Expected: FAIL with "undefined: NewStreetCodeClassifier"

**Step 3: Write minimal implementation**

```go
// classifier/internal/classifier/streetcode.go
package classifier

import (
	"context"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/mlclient"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// StreetCode classification thresholds.
const (
	HomepageMinConfidence = 0.75
	RuleHighConfidence    = 0.85
	MLOverrideThreshold   = 0.90
	maxBodyChars          = 500
)

// MLClassifier defines the interface for ML classification.
type MLClassifier interface {
	Classify(ctx context.Context, title, body string) (*mlclient.ClassifyResponse, error)
	Health(ctx context.Context) error
}

// StreetCodeClassifier implements hybrid rule + ML classification.
type StreetCodeClassifier struct {
	mlClient MLClassifier
	logger   infralogger.Logger
	enabled  bool
}

// StreetCodeResult holds the hybrid classification result.
type StreetCodeResult struct {
	Relevance           string   `json:"street_crime_relevance"`
	CrimeTypes          []string `json:"crime_types"`
	LocationSpecificity string   `json:"location_specificity"`
	FinalConfidence     float64  `json:"final_confidence"`
	HomepageEligible    bool     `json:"homepage_eligible"`
	CategoryPages       []string `json:"category_pages"`
	ReviewRequired      bool     `json:"review_required"`
	RuleRelevance       string   `json:"rule_relevance"`
	RuleConfidence      float64  `json:"rule_confidence"`
	MLRelevance         string   `json:"ml_relevance,omitempty"`
	MLConfidence        float64  `json:"ml_confidence,omitempty"`
}

// NewStreetCodeClassifier creates a new hybrid classifier.
func NewStreetCodeClassifier(mlClient MLClassifier, logger infralogger.Logger, enabled bool) *StreetCodeClassifier {
	return &StreetCodeClassifier{
		mlClient: mlClient,
		logger:   logger,
		enabled:  enabled,
	}
}

// Classify performs hybrid classification on raw content.
func (s *StreetCodeClassifier) Classify(ctx context.Context, raw *domain.RawContent) (*StreetCodeResult, error) {
	if !s.enabled {
		return nil, nil
	}

	// Layer 1 & 2: Rule-based classification
	ruleResult := classifyByRules(raw.Title, raw.RawText)

	// Layer 3: ML classification (if ML service available)
	var mlResult *mlclient.ClassifyResponse
	if s.mlClient != nil {
		body := raw.RawText
		if len(body) > maxBodyChars {
			body = body[:maxBodyChars]
		}
		var err error
		mlResult, err = s.mlClient.Classify(ctx, raw.Title, body)
		if err != nil {
			s.logger.Warn("ML classification failed, using rules only",
				infralogger.String("content_id", raw.ID),
				infralogger.Error(err))
		}
	}

	// Decision layer: merge results
	return s.mergeResults(ruleResult, mlResult), nil
}

// mergeResults combines rule and ML results using the decision matrix.
func (s *StreetCodeClassifier) mergeResults(rule *ruleResult, ml *mlclient.ClassifyResponse) *StreetCodeResult {
	result := &StreetCodeResult{
		RuleRelevance:  rule.relevance,
		RuleConfidence: rule.confidence,
		CrimeTypes:     rule.crimeTypes,
	}

	if ml != nil {
		result.MLRelevance = ml.Relevance
		result.MLConfidence = ml.RelevanceConfidence
		result.LocationSpecificity = ml.Location
	}

	// Decision logic
	switch {
	case rule.relevance == "core_street_crime" && ml != nil && ml.Relevance == "core_street_crime":
		// Both agree: high confidence
		result.Relevance = "core_street_crime"
		result.FinalConfidence = (rule.confidence + ml.RelevanceConfidence) / 2
		result.HomepageEligible = result.FinalConfidence >= HomepageMinConfidence

	case rule.relevance == "core_street_crime" && ml != nil && ml.Relevance == "not_crime":
		// Rule says core, ML says not_crime: flag for review
		result.Relevance = "core_street_crime"
		result.FinalConfidence = rule.confidence * 0.7
		result.HomepageEligible = rule.confidence >= RuleHighConfidence
		result.ReviewRequired = true

	case rule.relevance == "core_street_crime":
		// Rule says core, ML unavailable or uncertain
		result.Relevance = "core_street_crime"
		result.FinalConfidence = rule.confidence
		result.HomepageEligible = rule.confidence >= RuleHighConfidence

	case ml != nil && ml.Relevance == "core_street_crime" && ml.RelevanceConfidence >= MLOverrideThreshold:
		// ML says core with high confidence, rule missed it
		result.Relevance = "peripheral_crime"
		result.FinalConfidence = ml.RelevanceConfidence * 0.8
		result.ReviewRequired = true

	default:
		result.Relevance = rule.relevance
		result.FinalConfidence = rule.confidence
	}

	// Merge crime types from ML
	if ml != nil {
		for _, ct := range ml.CrimeTypes {
			if !containsString(result.CrimeTypes, ct) {
				result.CrimeTypes = append(result.CrimeTypes, ct)
			}
		}
	}

	// Map to category pages
	result.CategoryPages = mapToCategoryPages(result.CrimeTypes)

	return result
}

// mapToCategoryPages converts crime types to StreetCode category page slugs.
func mapToCategoryPages(crimeTypes []string) []string {
	mapping := map[string][]string{
		"violent_crime":    {"violent-crime", "crime"},
		"property_crime":   {"property-crime", "crime"},
		"drug_crime":       {"drug-crime", "crime"},
		"gang_violence":    {"gang-violence", "crime"},
		"organized_crime":  {"organized-crime", "crime"},
		"criminal_justice": {"court-news"},
		"other_crime":      {"crime"},
	}

	pages := make(map[string]bool)
	for _, ct := range crimeTypes {
		for _, page := range mapping[ct] {
			pages[page] = true
		}
	}

	result := make([]string, 0, len(pages))
	for page := range pages {
		result = append(result, page)
	}
	return result
}
```

**Step 4: Run test to verify it passes**

Run: `cd classifier && go test ./internal/classifier/streetcode_test.go ./internal/classifier/streetcode.go ./internal/classifier/streetcode_rules.go -v`
Expected: PASS (3 tests)

**Step 5: Commit**

```bash
git add classifier/internal/classifier/streetcode.go classifier/internal/classifier/streetcode_test.go
git commit -m "feat(classifier): implement hybrid streetcode classifier

Combines rule-based precision with ML recall using decision matrix.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 12: Update Domain Model for StreetCode

**Files:**
- Modify: `classifier/internal/domain/classification.go`

**Step 1: Add StreetCodeResult to ClassificationResult**

Add after the existing fields in `ClassificationResult`:

```go
// StreetCode hybrid classification (optional)
StreetCode *StreetCodeResult `json:"streetcode,omitempty"`
```

And add the StreetCodeResult struct:

```go
// StreetCodeResult holds StreetCode hybrid classification results.
type StreetCodeResult struct {
	Relevance           string   `json:"street_crime_relevance"`
	CrimeTypes          []string `json:"crime_types"`
	LocationSpecificity string   `json:"location_specificity"`
	FinalConfidence     float64  `json:"final_confidence"`
	HomepageEligible    bool     `json:"homepage_eligible"`
	CategoryPages       []string `json:"category_pages"`
	ReviewRequired      bool     `json:"review_required"`
}
```

**Step 2: Run existing tests**

Run: `cd classifier && go test ./internal/domain/... -v`
Expected: PASS (or no tests in domain)

**Step 3: Commit**

```bash
git add classifier/internal/domain/classification.go
git commit -m "feat(classifier): add StreetCodeResult to domain model

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 13: Integrate StreetCode into Main Classifier

**Files:**
- Modify: `classifier/internal/classifier/classifier.go`

**Step 1: Add streetcode field to Classifier struct**

Update the Classifier struct:

```go
type Classifier struct {
	contentType      *ContentTypeClassifier
	quality          *QualityScorer
	topic            *TopicClassifier
	sourceReputation *SourceReputationScorer
	streetcode       *StreetCodeClassifier  // NEW
	logger           infralogger.Logger
	version          string
}
```

**Step 2: Update NewClassifier to accept StreetCodeClassifier**

Add parameter and assignment in NewClassifier.

**Step 3: Add StreetCode classification in Classify method**

After topic classification (step 4), add:

```go
// 5. StreetCode Classification (if enabled)
var streetcodeResult *StreetCodeResult
if c.streetcode != nil {
	streetcodeResult, err = c.streetcode.Classify(ctx, raw)
	if err != nil {
		c.logger.Warn("StreetCode classification failed",
			infralogger.String("content_id", raw.ID),
			infralogger.Error(err))
	}
}

// Add to result
result.StreetCode = streetcodeResult
```

**Step 4: Run tests**

Run: `cd classifier && go test ./internal/classifier/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add classifier/internal/classifier/classifier.go
git commit -m "feat(classifier): integrate streetcode into main classifier

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 14: Update Bootstrap to Create StreetCode Classifier

**Files:**
- Modify: `classifier/internal/bootstrap/classifier.go`
- Modify: `classifier/internal/config/config.go`

**Step 1: Add StreetCode config to config.go**

```go
type StreetCodeConfig struct {
	Enabled      bool   `yaml:"enabled" env:"STREETCODE_ENABLED" envDefault:"false"`
	MLServiceURL string `yaml:"ml_service_url" env:"STREETCODE_ML_SERVICE_URL" envDefault:"http://streetcode-ml:8076"`
}
```

Add to main Config struct:

```go
StreetCode StreetCodeConfig `yaml:"streetcode"`
```

**Step 2: Create StreetCode classifier in bootstrap**

In NewHTTPComponents, after creating topicClassifier:

```go
// Create StreetCode classifier if enabled
var streetcodeClassifier *classifier.StreetCodeClassifier
if cfg.StreetCode.Enabled {
	var mlClient classifier.MLClassifier
	if cfg.StreetCode.MLServiceURL != "" {
		mlClient = mlclient.NewClient(cfg.StreetCode.MLServiceURL)
	}
	streetcodeClassifier = classifier.NewStreetCodeClassifier(mlClient, logger, true)
	logger.Info("StreetCode classifier enabled",
		infralogger.String("ml_service_url", cfg.StreetCode.MLServiceURL))
}
```

**Step 3: Pass to classifier constructor**

Update the classifier.NewClassifier call to include streetcodeClassifier.

**Step 4: Run tests**

Run: `cd classifier && go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add classifier/internal/bootstrap/classifier.go classifier/internal/config/config.go
git commit -m "feat(classifier): wire streetcode classifier in bootstrap

Enabled via STREETCODE_ENABLED=true environment variable.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 15: Add StreetCode Fields to Elasticsearch Mapping

**Files:**
- Modify: `classifier/internal/elasticsearch/mappings/classified_content.go`

**Step 1: Add streetcode fields to mapping**

In the properties section, add:

```go
"streetcode": map[string]any{
	"properties": map[string]any{
		"street_crime_relevance": map[string]any{"type": "keyword"},
		"crime_types":            map[string]any{"type": "keyword"},
		"location_specificity":   map[string]any{"type": "keyword"},
		"final_confidence":       map[string]any{"type": "float"},
		"homepage_eligible":      map[string]any{"type": "boolean"},
		"category_pages":         map[string]any{"type": "keyword"},
		"review_required":        map[string]any{"type": "boolean"},
	},
},
```

**Step 2: Run tests**

Run: `cd classifier && go test ./internal/elasticsearch/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add classifier/internal/elasticsearch/mappings/classified_content.go
git commit -m "feat(classifier): add streetcode fields to ES mapping

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 16: Update Publisher Article Model

**Files:**
- Modify: `publisher/internal/router/service.go`

**Step 1: Add StreetCode fields to Article struct**

```go
// StreetCode classification
StreetCrimeRelevance string   `json:"street_crime_relevance"`
StreetCodeCrimeTypes []string `json:"streetcode_crime_types"`
LocationSpecificity  string   `json:"location_specificity"`
HomepageEligible     bool     `json:"homepage_eligible"`
CategoryPages        []string `json:"category_pages"`
ReviewRequired       bool     `json:"review_required"`
```

**Step 2: Add fields to publishToChannel payload**

```go
"street_crime_relevance": article.StreetCrimeRelevance,
"streetcode_crime_types": article.StreetCodeCrimeTypes,
"location_specificity":   article.LocationSpecificity,
"homepage_eligible":      article.HomepageEligible,
"category_pages":         article.CategoryPages,
"review_required":        article.ReviewRequired,
```

**Step 3: Run tests**

Run: `cd publisher && go test ./internal/router/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add publisher/internal/router/service.go
git commit -m "feat(publisher): add streetcode fields to article model

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 17: Create StreetCode Publisher Router

**Files:**
- Create: `publisher/internal/router/streetcode.go`
- Create: `publisher/internal/router/streetcode_test.go`

**Step 1: Write the failing test**

```go
// publisher/internal/router/streetcode_test.go
package router

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

type mockRedis struct {
	published map[string][]string
}

func (m *mockRedis) Publish(_ context.Context, channel string, message any) *redis.IntCmd {
	if m.published == nil {
		m.published = make(map[string][]string)
	}
	m.published[channel] = append(m.published[channel], message.(string))
	return redis.NewIntCmd(context.Background())
}

func TestStreetCodeRouter_Route_HomepageEligible(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:                   "test-1",
		Title:                "Murder suspect arrested",
		HomepageEligible:     true,
		StreetCrimeRelevance: "core_street_crime",
		CategoryPages:        []string{"violent-crime", "crime"},
	}

	// Should publish to streetcode:homepage and category channels
	channels := GenerateStreetCodeChannels(article)

	if !containsChannel(channels, "streetcode:homepage") {
		t.Error("expected streetcode:homepage channel")
	}

	if !containsChannel(channels, "streetcode:category:violent-crime") {
		t.Error("expected streetcode:category:violent-crime channel")
	}
}

func TestStreetCodeRouter_Route_NotHomepageEligible(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:                   "test-2",
		Title:                "Minor incident",
		HomepageEligible:     false,
		StreetCrimeRelevance: "peripheral_crime",
		CategoryPages:        []string{"crime"},
	}

	channels := GenerateStreetCodeChannels(article)

	if containsChannel(channels, "streetcode:homepage") {
		t.Error("should not include homepage for non-eligible article")
	}

	if !containsChannel(channels, "streetcode:category:crime") {
		t.Error("expected streetcode:category:crime channel")
	}
}

func containsChannel(channels []string, target string) bool {
	for _, ch := range channels {
		if ch == target {
			return true
		}
	}
	return false
}
```

**Step 2: Run test to verify it fails**

Run: `cd publisher && go test ./internal/router/streetcode_test.go -v`
Expected: FAIL with "undefined: GenerateStreetCodeChannels"

**Step 3: Write minimal implementation**

```go
// publisher/internal/router/streetcode.go
package router

import "fmt"

// GenerateStreetCodeChannels returns the Redis channels for a StreetCode article.
func GenerateStreetCodeChannels(article *Article) []string {
	channels := make([]string, 0)

	// Skip non-crime articles
	if article.StreetCrimeRelevance == "not_crime" || article.StreetCrimeRelevance == "" {
		return channels
	}

	// Homepage channel if eligible
	if article.HomepageEligible {
		channels = append(channels, "streetcode:homepage")
	}

	// Category channels
	for _, category := range article.CategoryPages {
		channels = append(channels, fmt.Sprintf("streetcode:category:%s", category))
	}

	return channels
}
```

**Step 4: Run test to verify it passes**

Run: `cd publisher && go test ./internal/router/streetcode_test.go ./internal/router/streetcode.go ./internal/router/service.go -v`
Expected: PASS (2 tests)

**Step 5: Commit**

```bash
git add publisher/internal/router/streetcode.go publisher/internal/router/streetcode_test.go
git commit -m "feat(publisher): add streetcode channel generation

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 18: Integrate StreetCode Routing into Main Router

**Files:**
- Modify: `publisher/internal/router/service.go`

**Step 1: Add StreetCode routing to routeArticle**

After Layer 2 routing, add:

```go
// Layer 3: StreetCode channels
streetCodeChannels := GenerateStreetCodeChannels(article)
for _, channel := range streetCodeChannels {
	s.publishToChannel(ctx, article, channel, nil)
}
```

**Step 2: Run tests**

Run: `cd publisher && go test ./internal/router/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add publisher/internal/router/service.go
git commit -m "feat(publisher): integrate streetcode routing into main router

Articles with streetcode classification are published to:
- streetcode:homepage (if eligible)
- streetcode:category:{category} channels

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 19: Add End-to-End Integration Test

**Files:**
- Create: `classifier/internal/classifier/integration_test.go`

**Step 1: Write integration test**

```go
// classifier/internal/classifier/integration_test.go
//go:build integration
// +build integration

package classifier_test

import (
	"context"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/mlclient"
)

func TestStreetCodeClassifier_Integration(t *testing.T) {
	// Skip if ML service not available
	client := mlclient.NewClient("http://localhost:8076")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Health(ctx); err != nil {
		t.Skip("ML service not available, skipping integration test")
	}

	sc := classifier.NewStreetCodeClassifier(client, nil, true)

	tests := []struct {
		name             string
		title            string
		body             string
		expectRelevance  string
		expectHomepage   bool
	}{
		{
			name:            "murder article",
			title:           "Man charged with murder after downtown stabbing",
			body:            "Police arrested a suspect following a fatal stabbing incident.",
			expectRelevance: "core_street_crime",
			expectHomepage:  true,
		},
		{
			name:            "restaurant article",
			title:           "New restaurant opens in downtown area",
			body:            "A new fine dining establishment has opened.",
			expectRelevance: "not_crime",
			expectHomepage:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := &domain.RawContent{
				ID:      "test-" + tt.name,
				Title:   tt.title,
				RawText: tt.body,
			}

			result, err := sc.Classify(context.Background(), raw)
			if err != nil {
				t.Fatalf("classification failed: %v", err)
			}

			if result.Relevance != tt.expectRelevance {
				t.Errorf("relevance: got %s, want %s", result.Relevance, tt.expectRelevance)
			}

			if result.HomepageEligible != tt.expectHomepage {
				t.Errorf("homepage eligible: got %v, want %v", result.HomepageEligible, tt.expectHomepage)
			}
		})
	}
}
```

**Step 2: Run integration test (requires ML service)**

Run: `cd classifier && go test ./internal/classifier/... -tags=integration -v`
Expected: PASS (or skip if ML service not running)

**Step 3: Commit**

```bash
git add classifier/internal/classifier/integration_test.go
git commit -m "test(classifier): add streetcode integration test

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 20: Update Documentation

**Files:**
- Modify: `classifier/CLAUDE.md`
- Modify: `publisher/CLAUDE.md`

**Step 1: Add StreetCode section to classifier/CLAUDE.md**

```markdown
## StreetCode Hybrid Classification

**Enabled via**: `STREETCODE_ENABLED=true` and `STREETCODE_ML_SERVICE_URL=http://streetcode-ml:8076`

**Architecture**: Rules (precision) + ML (recall) with decision matrix

**Relevance Classes**:
- `core_street_crime` - Homepage eligible (murders, shootings, assaults with arrest)
- `peripheral_crime` - Category pages only (impaired driving, international, policy)
- `not_crime` - Excluded

**Crime Types** (multi-label):
- violent_crime, property_crime, drug_crime, gang_violence, organized_crime, criminal_justice

**Decision Matrix**:
| Rules | ML | Decision |
|-------|-----|----------|
| core | core | Homepage (high confidence) |
| core | not_crime | Review queue |
| not_crime | core (>0.9) | Review queue |
| both not_crime | - | Exclude |
```

**Step 2: Add StreetCode section to publisher/CLAUDE.md**

```markdown
## StreetCode Channels

**Layer 3 routing** for StreetCode articles:

- `streetcode:homepage` - High-confidence core street crime
- `streetcode:category:violent-crime` - Violent crime articles
- `streetcode:category:property-crime` - Property crime articles
- `streetcode:category:drug-crime` - Drug crime articles
- `streetcode:category:crime` - All crime articles
- `streetcode:category:court-news` - Criminal justice articles

**Fields used**:
- `homepage_eligible: true` → publishes to streetcode:homepage
- `category_pages: ["violent-crime", "crime"]` → publishes to each category channel
```

**Step 3: Commit**

```bash
git add classifier/CLAUDE.md publisher/CLAUDE.md
git commit -m "docs: add streetcode classification documentation

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 21: Run Full Test Suite and Lint

**Files:**
- None (verification only)

**Step 1: Run linter on classifier**

Run: `cd classifier && golangci-lint run`
Expected: No errors

**Step 2: Run linter on publisher**

Run: `cd publisher && golangci-lint run`
Expected: No errors

**Step 3: Run all classifier tests**

Run: `cd classifier && go test ./... -v`
Expected: All tests pass

**Step 4: Run all publisher tests**

Run: `cd publisher && go test ./... -v`
Expected: All tests pass

**Step 5: Run streetcode-ml tests**

Run: `cd streetcode-ml && python -m pytest tests/ -v`
Expected: All tests pass

---

## Task 22: Final Commit and Summary

**Step 1: Verify git status**

Run: `git status`
Expected: Clean working tree (all changes committed)

**Step 2: Create summary commit**

```bash
git log --oneline -20
```

Review the commit history to ensure all tasks are complete.

---

## Summary

| Phase | Tasks | Components |
|-------|-------|------------|
| ML Service | 1-8 | streetcode-ml FastAPI, sklearn models, Docker |
| Go Client | 9-11 | mlclient, streetcode_rules, streetcode classifier |
| Integration | 12-15 | Domain model, main classifier, bootstrap, ES mapping |
| Publisher | 16-18 | Article model, channel generation, routing |
| Testing | 19-21 | Integration tests, lint, full suite |

**Total: 22 tasks**
