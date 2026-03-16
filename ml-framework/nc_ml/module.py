"""Abstract base classes for ML sidecar modules."""

from abc import ABC, abstractmethod

from nc_ml.schemas import ClassifierResult, ClassifyRequest, ExtractorResult


class BaseModule(ABC):
    """Base class for all ML sidecar modules."""

    @abstractmethod
    def name(self) -> str:
        """Return the module name."""

    @abstractmethod
    def version(self) -> str:
        """Return the module version."""

    @abstractmethod
    def schema_version(self) -> str:
        """Return the schema version."""

    @abstractmethod
    async def initialize(self) -> None:
        """Initialize the module (load models, etc.)."""

    @abstractmethod
    async def shutdown(self) -> None:
        """Clean up resources."""

    @abstractmethod
    async def health_checks(self) -> dict[str, bool]:
        """Return a dict of check_name -> pass/fail."""


class ClassifierModule(BaseModule):
    """Module that classifies content."""

    @abstractmethod
    async def classify(self, request: ClassifyRequest) -> ClassifierResult:
        """Classify the given request."""


class ExtractorModule(BaseModule):
    """Module that extracts structured data from content."""

    @abstractmethod
    async def extract(self, request: ClassifyRequest) -> ExtractorResult:
        """Extract data from the given request."""
