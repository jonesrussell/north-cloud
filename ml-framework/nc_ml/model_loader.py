"""Model loading utilities using joblib."""

from pathlib import Path
from typing import Any

import joblib


class ModelLoadError(Exception):
    """Raised when a model file cannot be loaded."""


def load_model(path: Path) -> Any:
    """Load a single joblib model file.

    Raises ModelLoadError if the file does not exist.
    """
    if not path.exists():
        msg = f"Model file not found: {path}"
        raise ModelLoadError(msg)
    return joblib.load(path)


def load_models_from_dir(directory: Path) -> dict[str, Any]:
    """Load all .joblib files from a directory.

    Returns a dict mapping filename stem to loaded model.
    """
    if not directory.exists():
        msg = f"Model directory not found: {directory}"
        raise ModelLoadError(msg)

    models: dict[str, Any] = {}
    for path in sorted(directory.glob("*.joblib")):
        models[path.stem] = joblib.load(path)
    return models
