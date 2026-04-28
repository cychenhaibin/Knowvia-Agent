class QQALLMError(Exception):
    """Base error for the LLM service."""


class ValidationError(QQALLMError):
    """Raised when a request payload is invalid."""


class IndexingError(QQALLMError):
    """Raised when indexing fails."""


class RetrievalError(QQALLMError):
    """Raised when retrieval fails."""


class GenerationError(QQALLMError):
    """Raised when generation fails."""


class SkillResolveError(QQALLMError):
    """Raised when skill resolution fails."""


class ExternalProviderError(QQALLMError):
    """Raised when provider payload parsing fails."""
