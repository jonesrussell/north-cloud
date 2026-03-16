"""Dynamic module loader entry point for ML sidecars.

Reads MODULE_NAME from environment, imports the module from /opt/module,
and creates the FastAPI app.
"""

import importlib
import os
import sys
import warnings

from nc_ml.app import create_app


def _load_module():
    module_name = os.environ.get("MODULE_NAME", "")  # noqa: SIM112
    if not module_name:
        msg = "MODULE_NAME environment variable is required"
        raise RuntimeError(msg)

    # Add module directory to path
    module_dir = "/opt/module"
    if module_dir not in sys.path:
        sys.path.insert(0, module_dir)

    # Import the module's entry point
    mod = importlib.import_module(module_name)
    if not hasattr(mod, "create_module"):
        msg = f"Module {module_name} must define a create_module() function"
        raise RuntimeError(msg)

    return mod.create_module()


app = None


def get_app():
    """Lazily create the app."""
    global app  # noqa: PLW0603
    if app is None:
        module = _load_module()
        app = create_app(module)
    return app


# uvicorn expects this at module level — but we defer creation
# so MODULE_NAME must be set before import
try:
    app = get_app()
except (RuntimeError, ModuleNotFoundError) as exc:
    if os.environ.get("MODULE_NAME"):  # noqa: SIM112
        warnings.warn(
            f"Failed to load module '{os.environ.get('MODULE_NAME')}': {exc}",  # noqa: SIM112
            RuntimeWarning,
            stacklevel=1,
        )
    # Allow import without MODULE_NAME for testing/import scenarios
