"""Structured logging configuration for ML sidecars."""

import logging

import structlog


def configure_logging(service_name: str, log_level: str = "info") -> None:
    """Configure structlog with JSON output and UTC timestamps."""
    numeric_level = getattr(logging, log_level.upper(), logging.INFO)
    structlog.configure(
        processors=[
            structlog.contextvars.merge_contextvars,
            structlog.processors.add_log_level,
            structlog.processors.TimeStamper(fmt="iso", utc=True),
            structlog.processors.StackInfoRenderer(),
            structlog.processors.format_exc_info,
            structlog.processors.JSONRenderer(),
        ],
        wrapper_class=structlog.make_filtering_bound_logger(numeric_level),
        context_class=dict,
        logger_factory=structlog.PrintLoggerFactory(),
        cache_logger_on_first_use=False,
    )


def get_logger(module: str, **kwargs: object) -> structlog.stdlib.BoundLogger:
    """Return a bound logger with module context."""
    return structlog.get_logger(module=module, **kwargs)
