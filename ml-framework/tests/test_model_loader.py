"""Tests for nc_ml.model_loader."""

from pathlib import Path

import joblib
import pytest

from nc_ml.model_loader import ModelLoadError, load_model, load_models_from_dir


def test_load_model_success(tmp_path):
    model_data = {"type": "test_model", "weights": [1, 2, 3]}
    path = tmp_path / "model.joblib"
    joblib.dump(model_data, path)

    loaded = load_model(path)
    assert loaded == model_data


def test_load_model_missing_file():
    with pytest.raises(ModelLoadError, match="not found"):
        load_model(Path("/nonexistent/model.joblib"))


def test_load_models_from_dir(tmp_path):
    joblib.dump({"a": 1}, tmp_path / "alpha.joblib")
    joblib.dump({"b": 2}, tmp_path / "beta.joblib")

    models = load_models_from_dir(tmp_path)
    assert set(models.keys()) == {"alpha", "beta"}
    assert models["alpha"] == {"a": 1}
    assert models["beta"] == {"b": 2}


def test_load_models_from_empty_dir(tmp_path):
    models = load_models_from_dir(tmp_path)
    assert models == {}


def test_load_models_from_missing_dir():
    with pytest.raises(ModelLoadError, match="not found"):
        load_models_from_dir(Path("/nonexistent/dir"))
