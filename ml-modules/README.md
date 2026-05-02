# ml-modules — per-topic classifier modules

Source-of-truth implementations of the `ClassifierModule` protocol from `ml-framework/nc_ml`. Each subdirectory is one ML domain (mining, crime, indigenous, entertainment, coforge) and is **mounted into the corresponding ML sidecar container at `/opt/module`** in dev.

## What it is

Five topic-specific Python packages, each exposing a `ClassifierModule` subclass that the `nc_ml` framework loads dynamically at sidecar startup. Each module owns its own:

- ML model artifacts (joblib files under `models/`)
- Per-classifier code (e.g., `classifier/relevance.py`, `classifier/commodity.py`)
- Pydantic result schemas extending `ClassifierResult`
- `requirements.txt` or `pyproject.toml` for module-specific deps (typically `scikit-learn`, `joblib`)

## Architecture

This directory is the topic-modules half of a 3-tier ML pipeline:

```
ml-framework/        → shared FastAPI framework + module protocol
ml-modules/{topic}/  → per-topic source-of-truth ClassifierModule implementations (this dir)
ml-sidecars/{topic}-ml/ → production Docker build contexts (embed the module code at build time)
```

In **dev** (`docker-compose.dev.yml` `ml` profile), each sidecar mounts both `./ml-modules/{topic}:/opt/module` and `./ml-framework:/opt/ml-framework`, then `nc_ml.main` reads `MODULE_NAME=<topic>` and dynamically imports the module from `/opt/module`. Edits here take effect on container restart, no image rebuild.

In **production**, the corresponding `ml-sidecars/{topic}-ml/Dockerfile` copies the module's code into the image and installs `nc_ml` as a regular dependency. The `ml-sidecars/{topic}-ml/module.py` files are build artifacts that mirror `ml-modules/{topic}/module.py`.

## Subdirectories

| Topic | Purpose | Sidecar pair |
|---|---|---|
| `coforge/` | CoForge classification (publisher routing layer L8) | `ml-sidecars/coforge-ml` |
| `crime/` | Street-crime classification (publisher routing layer L3) | `ml-sidecars/crime-ml` |
| `entertainment/` | Entertainment classification (publisher routing layer L6) | `ml-sidecars/entertainment-ml` |
| `indigenous/` | Indigenous classification (publisher routing layer L7) | `ml-sidecars/indigenous-ml` |
| `mining/` | Mining classification (publisher routing layer L5) | `ml-sidecars/mining-ml` |

Each subdirectory typically contains:

```
ml-modules/{topic}/
├── __init__.py
├── module.py              # ClassifierModule subclass; framework entry point
├── classifier/            # per-classifier algorithms (one .py per classifier)
├── models/                # joblib model artifacts
└── requirements.txt OR pyproject.toml
```

## Adding a new topic module

Per root `CLAUDE.md`:

1. Create `ml-modules/<name>/` following the layout above
2. Add `<NAME>_ENABLED` and `<NAME>_ML_SERVICE_URL` to `classifier/internal/config/`
3. Add a routing layer in `publisher/`
4. Add the dev sidecar service to `docker-compose.dev.yml` under the `ml` profile (mount both `./ml-modules/<name>` and `./ml-framework`)
5. Create `ml-sidecars/<name>-ml/` build context with Dockerfile

See `classifier/CLAUDE.md` and `ml-sidecars/README.md` for the API contract.

## Drift note

The `module.py` in `ml-modules/{topic}/` and `ml-sidecars/{topic}-ml/` are intended to mirror each other. If they drift, the sidecar's image rebuild will use the `ml-sidecars/` copy in production while dev runs from `ml-modules/`. Tracking this drift is out of scope for the current audit; consider a follow-up to auto-sync them or treat one as the canonical source.
