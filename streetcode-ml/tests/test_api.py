# streetcode-ml/tests/test_api.py
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


class TestClassifyEndpoint:
    def test_classify_returns_all_fields(self, client):
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

    def test_classify_with_empty_body(self, client):
        response = client.post("/classify", json={
            "title": "Man charged with murder",
            "body": ""
        })

        assert response.status_code == 200

    def test_classify_with_missing_body(self, client):
        response = client.post("/classify", json={
            "title": "Man charged with murder"
        })

        assert response.status_code == 200
