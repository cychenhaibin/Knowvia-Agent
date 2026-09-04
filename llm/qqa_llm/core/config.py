from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path


def _env(name: str, default: str = "") -> str:
    return os.getenv(name, default).strip()


def _env_int(name: str, default: int) -> int:
    raw = os.getenv(name)
    if raw is None or not raw.strip():
        return default
    try:
        return int(raw.strip())
    except ValueError:
        return default


def _env_float(name: str, default: float) -> float:
    raw = os.getenv(name)
    if raw is None or not raw.strip():
        return default
    try:
        return float(raw.strip())
    except ValueError:
        return default


@dataclass(frozen=True)
class Settings:
    data_dir: Path
    internal_auth_token: str
    embedding_dim: int
    embedding_backend: str
    embedding_model: str
    bce_embedding_model: str
    lexical_top_k: int
    vector_top_k: int
    fusion_top_k: int
    rerank_top_k: int
    rerank_backend: str
    rerank_model: str
    bce_rerank_model: str
    max_sources: int
    chunk_size: int
    chunk_overlap: int
    max_snippet_chars: int
    generator_backend: str
    default_chat_model: str
    default_chat_quality_model: str
    default_report_model: str
    openai_api_base: str
    openai_api_key: str
    ollama_base_url: str
    ollama_model: str
    index_backend: str = "auto"
    postgres_dsn: str = ""
    postgres_schema: str = "qqa_llm"
    postgres_connect_timeout: int = 5
    trace_retention_days: int = 30
    report_trace_retention_days: int = 90
    default_chat_profile_id: str = "profile_chat_fast_default"
    default_chat_quality_profile_id: str = "profile_chat_quality_default"
    default_report_profile_id: str = "profile_report_default"
    default_chat_temperature: float = 0.05
    default_chat_quality_temperature: float = 0.05
    default_report_temperature: float = 0.1


def load_settings() -> Settings:
    return Settings(
        data_dir=Path(_env("QQA_LLM_DATA_DIR", "./data/internal")).resolve(),
        internal_auth_token=_env("QQA_PYTHON_PROXY_TOKEN", ""),
        embedding_dim=_env_int("QQA_EMBEDDING_DIM", 768),
        embedding_backend=_env("QQA_EMBEDDING_BACKEND", "bce").lower() or "bce",
        embedding_model=_env("QQA_EMBEDDING_MODEL", "text-embedding-3-small"),
        bce_embedding_model=_env("QQA_BCE_EMBEDDING_MODEL", "maidalun1020/bce-embedding-base_v1"),
        lexical_top_k=_env_int("QQA_LEXICAL_TOP_K", 24),
        vector_top_k=_env_int("QQA_VECTOR_TOP_K", 24),
        fusion_top_k=_env_int("QQA_FUSION_TOP_K", 20),
        rerank_top_k=_env_int("QQA_RERANK_TOP_K", 8),
        rerank_backend=_env("QQA_RERANK_BACKEND", "bce").lower() or "bce",
        rerank_model=_env("QQA_RERANK_MODEL", ""),
        bce_rerank_model=_env("QQA_BCE_RERANK_MODEL", "maidalun1020/bce-reranker-base_v1"),
        max_sources=_env_int("QQA_MAX_SOURCES", 8),
        chunk_size=_env_int("QQA_CHUNK_SIZE", 700),
        chunk_overlap=_env_int("QQA_CHUNK_OVERLAP", 120),
        max_snippet_chars=_env_int("QQA_MAX_SNIPPET_CHARS", 220),
        generator_backend=_env("QQA_GENERATOR_BACKEND", "auto").lower() or "auto",
        default_chat_model=_env("OPENAI_MODEL", "gpt-4.1-mini"),
        default_chat_quality_model=_env("OPENAI_CHAT_QUALITY_MODEL", _env("OPENAI_MODEL", "gpt-4.1-mini")),
        default_report_model=_env("OPENAI_REPORT_MODEL", _env("OPENAI_MODEL", "gpt-4.1-mini")),
        openai_api_base=_env("OPENAI_API_BASE", ""),
        openai_api_key=_env("OPENAI_API_KEY", ""),
        ollama_base_url=_env("OLLAMA_BASE_URL", "http://127.0.0.1:11434"),
        ollama_model=_env("OLLAMA_MODEL", "qwen2.5:7b"),
        index_backend=_env("QQA_INDEX_BACKEND", "auto").lower() or "auto",
        postgres_dsn=_env("QQA_POSTGRES_DSN", ""),
        postgres_schema=_env("QQA_POSTGRES_SCHEMA", "qqa_llm") or "qqa_llm",
        postgres_connect_timeout=_env_int("QQA_POSTGRES_CONNECT_TIMEOUT", 5),
        trace_retention_days=_env_int("QQA_TRACE_RETENTION_DAYS", 30),
        report_trace_retention_days=_env_int("QQA_REPORT_TRACE_RETENTION_DAYS", 90),
        default_chat_temperature=_env_float("QQA_DEFAULT_CHAT_TEMPERATURE", 0.05),
        default_chat_quality_temperature=_env_float("QQA_DEFAULT_CHAT_QUALITY_TEMPERATURE", 0.05),
        default_report_temperature=_env_float("QQA_DEFAULT_REPORT_TEMPERATURE", 0.1),
    )
