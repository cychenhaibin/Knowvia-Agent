from __future__ import annotations

from fastapi import APIRouter, Request

from qqa_llm.api.schemas import HealthResponse
from qqa_llm.inference.generator import describe_generation_backend
from qqa_llm.inference.embeddings import describe_embedding_backend
from qqa_llm.inference.reranker import describe_rerank_backend

router = APIRouter(tags=["health"])


def build_health_payload(container) -> HealthResponse:
    known_users = container.list_known_users()
    scope_count = sum(len(container.scope_service.list_scope_ids(user_id)) for user_id in known_users)
    db_status = container.index_backend.health_status()
    default_profile = container.model_profile_service.resolve(user_id="", purpose="chat_fast")
    generator_status = describe_generation_backend(container.settings, default_profile)
    embedding_status = describe_embedding_backend(container.settings)
    rerank_status = describe_rerank_backend(container.settings, container.retrieval_service.retrieval_pipeline.vector.embedding_client)
    quality_suites = container.quality_eval_service.list_suites()
    model_profile_statuses = {}
    degraded_reasons = []
    overall_status = "ok"
    for purpose, payload in container.model_profile_service.profile_statuses("").items():
        resolved = container.model_profile_service.resolve(user_id="", purpose=purpose)
        generation = describe_generation_backend(container.settings, resolved)
        model_profile_statuses[purpose] = {
            **payload,
            "status": generation.get("status") or "ok",
            "message": generation.get("message") or "",
        }
        if generation.get("status") == "degraded":
            overall_status = "degraded"
            degraded_reasons.append(
                f"model profile '{purpose}' is degraded: {generation.get('message') or 'generation backend is not ready'}"
            )
    if db_status.get("status") == "error":
        overall_status = "degraded"
        degraded_reasons.append("database backend reported an error")
    if db_status.get("status") == "degraded" and overall_status == "ok":
        overall_status = "degraded"
        degraded_reasons.append("database backend is degraded")
    if generator_status.get("status") == "degraded" and overall_status == "ok":
        overall_status = "degraded"
        degraded_reasons.append(str(generator_status.get("message") or "generator backend is degraded"))
    if embedding_status.get("status") == "degraded" and overall_status == "ok":
        overall_status = "degraded"
        degraded_reasons.append(str(embedding_status.get("message") or "embedding backend is degraded"))
    if rerank_status.get("status") == "degraded" and overall_status == "ok":
        overall_status = "degraded"
        degraded_reasons.append(str(rerank_status.get("message") or "rerank backend is degraded"))
    if str(db_status.get("vector_mode") or "") != "native":
        if overall_status == "ok":
            overall_status = "degraded"
        degraded_reasons.append("vector index is not using native pgvector execution")
    checks = {
        "database": str(db_status.get("status") or "unknown"),
        "vectorIndex": "ok" if str(db_status.get("vector_mode") or "") == "native" else "degraded",
        "embeddingModel": str(embedding_status.get("status") or "unknown"),
        "generator": str(generator_status.get("status") or "unknown"),
        "rerankModel": str(rerank_status.get("status") or "unknown"),
        "modelProfiles": "degraded" if any(item.get("status") == "degraded" for item in model_profile_statuses.values()) else "ok",
        "qualityBenchmarks": "ok" if quality_suites else "degraded",
    }
    return HealthResponse(
        status=overall_status,
        message="qqa_llm is running",
        checks=checks,
        degraded_reasons=list(dict.fromkeys(degraded_reasons)),
        scope_count=scope_count,
        index_backend=container.index_backend.backend_name(),
        generator_backend=str(generator_status.get("backend") or container.settings.generator_backend),
        db=db_status,
        generator=generator_status,
        embedding=embedding_status,
        rerank=rerank_status,
        model_profiles=model_profile_statuses,
    )


@router.get("/health", response_model=HealthResponse)
@router.get("/internal/v1/health", response_model=HealthResponse)
def health(request: Request) -> HealthResponse:
    return build_health_payload(request.app.state.container)
