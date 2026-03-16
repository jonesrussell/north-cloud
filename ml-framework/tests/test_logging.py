"""Tests for nc_ml.logging."""

import json

from nc_ml.logging import configure_logging, get_logger


def test_configure_logging_does_not_raise():
    configure_logging("test-service")


def test_get_logger_returns_logger():
    configure_logging("test-service")
    log = get_logger("my-module")
    assert log is not None


def test_output_is_valid_json(capsys):
    configure_logging("test-service")
    log = get_logger("my-module")
    log.info("hello")
    captured = capsys.readouterr()
    data = json.loads(captured.out.strip())
    assert "timestamp" in data
    assert data["module"] == "my-module"
    assert data["event"] == "hello"
