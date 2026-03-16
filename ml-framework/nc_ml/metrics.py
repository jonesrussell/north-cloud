"""Prometheus metrics for ML sidecars."""

from prometheus_client import Counter, Gauge, Histogram

# Counters
requests_total = Counter(
    "ncml_requests_total",
    "Total requests processed",
    ["module", "status"],
)

errors_total = Counter(
    "ncml_errors_total",
    "Total errors encountered",
    ["module", "error_code"],
)

# Histograms
request_duration_seconds = Histogram(
    "ncml_request_duration_seconds",
    "Request processing duration in seconds",
    ["module"],
)

prediction_duration_seconds = Histogram(
    "ncml_prediction_duration_seconds",
    "Model prediction duration in seconds",
    ["module"],
)

# Gauges
model_info = Gauge(
    "ncml_model_info",
    "Model metadata (always 1 when set)",
    ["module", "version", "schema_version"],
)

health_status = Gauge(
    "ncml_health_status",
    "Current health status (1 = active, 0 = inactive)",
    ["module", "status"],
)

_HEALTH_STATES = ("healthy", "degraded", "unhealthy")


def set_model_info(module: str, version: str, schema_version: str) -> None:
    """Set model info gauge."""
    model_info.labels(module=module, version=version, schema_version=schema_version).set(1)


def set_health(module: str, status: str) -> None:
    """Set current health status to 1, all others to 0."""
    for state in _HEALTH_STATES:
        health_status.labels(module=module, status=state).set(1 if state == status else 0)
