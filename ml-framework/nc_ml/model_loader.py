"""Model loading utilities using joblib.

Security note: joblib.load() uses pickle deserialization which can execute
arbitrary code. Only load model files from trusted sources. In production,
models are baked into Docker images at build time from CI artifacts — never
loaded from user-supplied paths or untrusted origins.
"""

from pathlib import Path
from typing import Any

import joblib

# Models must reside under this directory. Paths outside it are rejected
# to prevent path traversal attacks loading arbitrary pickle files.
_ALLOWED_MODEL_ROOT = Path("/opt/module/models")


class ModelLoadError(Exception):
    """Raised when a model file cannot be loaded."""


def _validate_model_path(path: Path) -> Path:
    """Ensure the resolved path is under the allowed model root.

    In development (when _ALLOWED_MODEL_ROOT doesn't exist), validation
    is skipped to allow loading from local project directories.
    """
    if not _ALLOWED_MODEL_ROOT.exists():
        return path
    resolved = path.resolve()
    if not resolved.is_relative_to(_ALLOWED_MODEL_ROOT.resolve()):
        msg = f"Model path {resolved} is outside allowed root {_ALLOWED_MODEL_ROOT}"
        raise ModelLoadError(msg)
    return resolved


def load_model(path: Path) -> Any:
    """Load a single joblib model file.

    WARNING: Uses pickle deserialization — only load trusted model files.
    Raises ModelLoadError if the file does not exist or is outside the allowed root.
    """
    safe_path = _validate_model_path(path)
    if not safe_path.exists():
        msg = f"Model file not found: {safe_path}"
        raise ModelLoadError(msg)
    return joblib.load(safe_path)


def load_models_from_dir(directory: Path) -> dict[str, Any]:
    """Load all .joblib files from a directory.

    WARNING: Uses pickle deserialization — only load trusted model files.
    Returns a dict mapping filename stem to loaded model.
    """
    safe_dir = _validate_model_path(directory)
    if not safe_dir.exists():
        msg = f"Model directory not found: {safe_dir}"
        raise ModelLoadError(msg)

    models: dict[str, Any] = {}
    for path in sorted(safe_dir.glob("*.joblib")):
        models[path.stem] = joblib.load(path)
    return models
