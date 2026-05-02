# ml-framework — `nc_ml`

Shared Python framework powering every ML sidecar in `ml-sidecars/`.

## What it is

A small FastAPI-based framework that handles the cross-cutting concerns every classification sidecar needs: app construction, model loading, request schemas, health checks, structured logging, Prometheus metrics, and a dynamic module-loader entry point. Each sidecar implements the `ClassifierModule` protocol from `nc_ml.module` and the framework wraps it into a uvicorn-served `POST /classify` + `GET /health` HTTP API.

Distributed as a local Python package (`pyproject.toml`, `name = "nc-ml"`, `version = "1.0.0"`).

## Architecture

This directory is the framework half of a 3-tier ML pipeline:

```
ml-framework/        → shared FastAPI framework + module protocol (this dir)
ml-modules/{topic}/  → per-topic source-of-truth ClassifierModule implementations
ml-sidecars/{topic}-ml/ → production Docker build contexts (embed the module code)
```

In **dev** (`docker-compose.dev.yml` `ml` profile), each sidecar container mounts both `./ml-modules/{topic}:/opt/module` and `./ml-framework:/opt/ml-framework`, then the framework's `nc_ml.main` entry point reads `MODULE_NAME=<topic>` from env and dynamically imports the module from `/opt/module`. This gives live-edit ergonomics for sidecar development without rebuilding images.

In **production**, sidecar images are built from `ml-sidecars/{topic}-ml/Dockerfile` with the topic module's code copied in directly — `nc_ml` is installed as a regular dependency.

## Layout

```
ml-framework/
├── pyproject.toml          # name = "nc-ml", version = "1.0.0"
└── nc_ml/
    ├── app.py              # FastAPI app factory
    ├── main.py             # Entry point: imports module by MODULE_NAME env, creates app
    ├── module.py           # Abstract base classes: BaseModule, ClassifierModule, ExtractorModule
    ├── schemas.py          # Pydantic request/response schemas (ClassifyRequest, ClassifierResult)
    ├── model_loader.py     # joblib model loading utilities
    ├── health.py           # Health-check aggregator
    ├── logging.py          # structlog setup
    ├── metrics.py          # prometheus-client integration
    └── middleware.py       # FastAPI middleware (request logging, error handling)
```

## Module protocol

Every sidecar's topic module subclasses `ClassifierModule` from `nc_ml.module`:

```python
from nc_ml.module import ClassifierModule
from nc_ml.schemas import ClassifyRequest, ClassifierResult

class MyTopicModule(ClassifierModule):
    def name(self) -> str: ...
    def version(self) -> str: ...
    def schema_version(self) -> str: ...
    async def initialize(self) -> None: ...      # load models
    async def shutdown(self) -> None: ...        # release resources
    async def health_checks(self) -> dict[str, bool]: ...
    async def classify(self, request: ClassifyRequest) -> ClassifierResult: ...
```

## Orchestration

For the consumer-facing API surface (`POST /classify` request schema, sidecar registration in the Classifier service, dev profiles in compose), see `ml-sidecars/README.md`.

For project-wide architecture context (charter, content pipeline layers, publisher routing), see the root `CLAUDE.md` and `docs/specs/classification.md`.
