from __future__ import annotations

from dataclasses import dataclass
from typing import Any, List, Optional
from fastapi import FastAPI

from qqa_llm.api.health import router as health_router
from qqa_llm.api.internal import router as internal_router
from qqa_llm.core.config import Settings, load_settings
from qqa_llm.core.logging import configure_logging
from qqa_llm.index.file_index import FileIndexBackend
from qqa_llm.index.pgvector_index import PGVectorIndexBackend
from qqa_llm.ingest.chunk import ChunkBuilder
from qqa_llm.ingest.normalize import DocumentNormalizer
from qqa_llm.inference.embeddings import build_embedding_client
from qqa_llm.inference.reranker import build_rerank_client
from qqa_llm.prompt.chat_builder import ChatPromptBuilder
from qqa_llm.prompt.report_builder import ReportPromptBuilder
from qqa_llm.retrieve.pipeline import RetrievalPipeline
from qqa_llm.services.chat_service import ChatService
from qqa_llm.services.evidence_service import EvidenceService
from qqa_llm.services.indexing_service import IndexingService
from qqa_llm.services.model_profile_service import ModelProfileService
from qqa_llm.services.quality_eval_service import QualityEvalService
from qqa_llm.services.report_service import ReportService
from qqa_llm.services.retrieval_service import RetrievalService
from qqa_llm.services.scope_service import ScopeService
from qqa_llm.services.skill_service import SkillService
from qqa_llm.storage.db import DatabaseAdapter
from qqa_llm.storage.file_repo import FileStore
from qqa_llm.storage.index_job_repo import DatabaseIndexJobRepository, IndexJobRepository
from qqa_llm.storage.model_profile_repo import DatabaseModelProfileRepository, ModelProfileRepository
from qqa_llm.storage.scope_repo import ScopeRepository
from qqa_llm.storage.skill_repo import DatabaseSkillRepository, SkillRepository
from qqa_llm.storage.trace_repo import DatabaseTraceRepository, TraceRepository


@dataclass
class ServiceContainer:
    settings: Settings
    index_backend: Any
    scope_service: ScopeService
    skill_service: SkillService
    model_profile_service: ModelProfileService
    indexing_service: IndexingService
    retrieval_service: RetrievalService
    evidence_service: EvidenceService
    chat_service: ChatService
    report_service: ReportService
    quality_eval_service: QualityEvalService
    trace_repo: Any
    index_job_repo: Any

    def list_known_users(self) -> List[str]:
        return self.scope_service.list_user_ids()


def build_container(settings: Settings) -> ServiceContainer:
    # File-backed storage remains the default developer path, while the
    # postgres backend moves the llm service closer to the target architecture
    # from the design docs without forcing callers to change payloads.
    file_store = FileStore(settings.data_dir)
    resolved_index_backend = settings.index_backend
    if resolved_index_backend == "auto":
        resolved_index_backend = "postgres" if settings.postgres_dsn else "file"
    if resolved_index_backend == "postgres":
        db = DatabaseAdapter(
            dsn=settings.postgres_dsn,
            schema=settings.postgres_schema,
            connect_timeout=settings.postgres_connect_timeout,
            embedding_dim=settings.embedding_dim,
        )
        index_backend = PGVectorIndexBackend(db)
        trace_repo = DatabaseTraceRepository(db)
        index_job_repo = DatabaseIndexJobRepository(db)
        skill_repo = DatabaseSkillRepository(db)
        model_profile_repo = DatabaseModelProfileRepository(db)
    else:
        skill_repo = SkillRepository(file_store, settings.data_dir / "skills.json")
        model_profile_repo = ModelProfileRepository(file_store, settings.data_dir / "model_profiles.json")
        knowledge_dir = settings.data_dir / "knowledge"
        scope_repo = ScopeRepository(file_store, knowledge_dir)
        index_backend = FileIndexBackend(scope_repo)
        trace_repo = TraceRepository(file_store, settings.data_dir / "traces.jsonl")
        index_job_repo = IndexJobRepository(file_store, settings.data_dir / "index_jobs.json")

    normalizer = DocumentNormalizer()
    chunk_builder = ChunkBuilder(settings=settings, normalizer=normalizer)
    embedding_client = build_embedding_client(settings)
    rerank_client = build_rerank_client(settings, embedding_client)
    retrieval_pipeline = RetrievalPipeline(settings, embedding_client, rerank_client)

    scope_service = ScopeService(index_backend)
    skill_service = SkillService(skill_repo)
    model_profile_service = ModelProfileService(settings, model_profile_repo)
    indexing_service = IndexingService(
        settings,
        index_backend=index_backend,
        index_job_repo=index_job_repo,
        normalizer=normalizer,
        chunk_builder=chunk_builder,
        embedding_client=embedding_client,
    )
    retrieval_service = RetrievalService(index_backend, retrieval_pipeline)
    evidence_service = EvidenceService(settings, trace_repo=trace_repo)
    chat_service = ChatService(
        settings,
        retrieval_service=retrieval_service,
        skill_service=skill_service,
        model_profile_service=model_profile_service,
        trace_repo=trace_repo,
        prompt_builder=ChatPromptBuilder(),
    )
    report_service = ReportService(
        settings,
        prompt_builder=ReportPromptBuilder(),
        model_profile_service=model_profile_service,
        trace_repo=trace_repo,
    )
    quality_eval_service = QualityEvalService(
        indexing_service=indexing_service,
        evidence_service=evidence_service,
        retrieval_service=retrieval_service,
        report_service=report_service,
        scope_service=scope_service,
        trace_repo=trace_repo,
    )

    return ServiceContainer(
        settings=settings,
        index_backend=index_backend,
        scope_service=scope_service,
        skill_service=skill_service,
        model_profile_service=model_profile_service,
        indexing_service=indexing_service,
        retrieval_service=retrieval_service,
        evidence_service=evidence_service,
        chat_service=chat_service,
        report_service=report_service,
        quality_eval_service=quality_eval_service,
        trace_repo=trace_repo,
        index_job_repo=index_job_repo,
    )


def create_app(settings: Optional[Settings] = None) -> FastAPI:
    configure_logging()
    resolved_settings = settings or load_settings()
    app = FastAPI(
        title="Knowvia LLM",
        description="Evidence reasoning engine for Knowvia.",
        version="2.0.0",
    )
    app.state.settings = resolved_settings
    app.state.container = build_container(resolved_settings)
    app.state.trace_cleanup = app.state.container.trace_repo.cleanup_retention(
        trace_retention_days=resolved_settings.trace_retention_days,
        report_trace_retention_days=resolved_settings.report_trace_retention_days,
    )
    app.state.container.indexing_service.resume_pending_jobs()
    app.include_router(health_router)
    app.include_router(internal_router)
    return app


app = create_app()
