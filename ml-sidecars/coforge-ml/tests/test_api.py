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
