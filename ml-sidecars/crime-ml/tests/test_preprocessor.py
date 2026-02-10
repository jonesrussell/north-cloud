# streetcode-ml/tests/test_preprocessor.py
import pytest
from classifier.preprocessor import preprocess_text


class TestPreprocessText:
    def test_lowercases_text(self):
        result = preprocess_text("POLICE ARREST Suspect")
        assert "police arrest suspect" in result

    def test_removes_urls(self):
        result = preprocess_text("Story at https://example.com/news")
        assert "https://" not in result
        assert "example.com" not in result

    def test_removes_emails(self):
        result = preprocess_text("Contact news@example.com for tips")
        assert "@" not in result

    def test_collapses_whitespace(self):
        result = preprocess_text("Multiple   spaces   here")
        assert "  " not in result

    def test_handles_empty_string(self):
        result = preprocess_text("")
        assert result == ""

    def test_handles_none(self):
        result = preprocess_text(None)
        assert result == ""
