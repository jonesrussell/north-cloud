"""Tests for nc_ml.module."""

import pytest

from nc_ml.module import BaseModule, ClassifierModule, ExtractorModule
from nc_ml.schemas import ClassifierResult, ClassifyRequest, ExtractorResult


def test_base_module_cannot_be_instantiated():
    with pytest.raises(TypeError):
        BaseModule()


def test_classifier_module_cannot_be_instantiated():
    with pytest.raises(TypeError):
        ClassifierModule()


def test_extractor_module_cannot_be_instantiated():
    with pytest.raises(TypeError):
        ExtractorModule()


class ConcreteClassifier(ClassifierModule):
    def name(self) -> str:
        return "test-classifier"

    def version(self) -> str:
        return "1.0.0"

    def schema_version(self) -> str:
        return "1"

    async def initialize(self) -> None:
        pass

    async def shutdown(self) -> None:
        pass

    async def health_checks(self) -> dict[str, bool]:
        return {"model_loaded": True}

    async def classify(self, request: ClassifyRequest) -> ClassifierResult:
        return ClassifierResult(relevance=0.9, confidence=0.8)


class ConcreteExtractor(ExtractorModule):
    def name(self) -> str:
        return "test-extractor"

    def version(self) -> str:
        return "1.0.0"

    def schema_version(self) -> str:
        return "1"

    async def initialize(self) -> None:
        pass

    async def shutdown(self) -> None:
        pass

    async def health_checks(self) -> dict[str, bool]:
        return {"ready": True}

    async def extract(self, request: ClassifyRequest) -> ExtractorResult:
        return ExtractorResult()


async def test_concrete_classifier_works():
    mod = ConcreteClassifier()
    assert mod.name() == "test-classifier"
    req = ClassifyRequest(title="T", body="B")
    result = await mod.classify(req)
    assert result.relevance == 0.9


async def test_concrete_extractor_works():
    mod = ConcreteExtractor()
    assert mod.name() == "test-extractor"
    req = ClassifyRequest(title="T", body="B")
    result = await mod.extract(req)
    assert isinstance(result, ExtractorResult)
