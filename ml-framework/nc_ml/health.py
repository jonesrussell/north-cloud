"""Health check aggregation logic."""


def aggregate_health_status(checks: dict[str, bool]) -> str:
    """Aggregate individual check results into an overall status.

    - All True or empty -> "healthy"
    - Some False -> "degraded"
    - All False -> "unhealthy"
    """
    if not checks:
        return "healthy"

    values = list(checks.values())
    if all(values):
        return "healthy"
    if not any(values):
        return "unhealthy"
    return "degraded"
