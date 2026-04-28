from __future__ import annotations

import json
from datetime import datetime, timezone
import uuid
from typing import Dict, Iterable

from fastapi import APIRouter, Depends, Request
from fastapi.responses import StreamingResponse

from qqa_llm.api.health import build_health_payload
from qqa_llm.api.schemas import (
    ChatStreamRequest,
    DeleteScopeRequest,
    DeleteScopeResponse,
    EvidenceMergeRequest,
    EvidenceMergeResponse,
    IndexJobResponse,
    IndexSyncSourceRequest,
    IndexUpsertBatchRequest,
    IndexUpsertBatchResponse,
    LegacyChatRequest,
    LegacyKnowledgeSourceSyncRequest,
    LegacyKnowledgeUpsertRequest,
    LegacyKnowledgeUpsertResponse,
    ModelProfileDeleteRequest,
    ModelProfileResponse,
    ModelProfileStatusResponse,
    ModelProfileUpsertRequest,
    QualityBenchmarkRunRequest,
    QualityBenchmarkRunResponse,
    QualitySuiteResponse,
    RequestTraceResponse,
    ReportGenerateRequest,
    ReportGenerateResponse,
    RetrieveRequest,
    RetrieveResponse,
    SkillDeleteRequest,
    SkillMirrorRequest,
    SkillStatusResponse,
)
from qqa_llm.auth.internal_auth import require_internal_client
from qqa_llm.domain.models import RetrievalResult

router = APIRouter(tags=["internal"])


def _container(request: Request):
    return request.app.state.container


def _sse(events: Iterable[Dict[str, object]]):
    for event in events:
        yield f"data: {json.dumps(event, ensure_ascii=False)}\n\n"


@router.post("/internal/v1/skills/upsert", response_model=SkillStatusResponse)
@router.post("/internal/skills/upsert")
def upsert_skill(
    req: SkillMirrorRequest,
    request: Request,
    _: str = Depends(require_internal_client),
):
    _container(request).skill_service.upsert(req.model_dump())
    return SkillStatusResponse(status="ok", skill_id=req.id)


@router.delete("/internal/v1/skills/{user_id}/{skill_id}", response_model=SkillStatusResponse)
@router.delete("/internal/skills/{user_id}/{skill_id}")
def delete_skill(user_id: str, skill_id: str, request: Request, _: str = Depends(require_internal_client)):
    _container(request).skill_service.delete(user_id, skill_id)
    return SkillStatusResponse(status="ok", skill_id=skill_id)


@router.delete("/internal/v1/skills/{skill_id}", response_model=SkillStatusResponse)
def delete_skill_v1_body(
    skill_id: str,
    req: SkillDeleteRequest,
    request: Request,
    _: str = Depends(require_internal_client),
):
    _container(request).skill_service.delete(req.user_id, skill_id)
    return SkillStatusResponse(status="ok", skill_id=skill_id)


@router.post("/internal/v1/model-profiles/upsert", response_model=ModelProfileStatusResponse)
def upsert_model_profile(
    req: ModelProfileUpsertRequest,
    request: Request,
    _: str = Depends(require_internal_client),
):
    payload = _container(request).model_profile_service.upsert(req.user_id, req.model_dump())
    return ModelProfileStatusResponse(status="ok", profile_id=str(payload.get("id") or ""))


@router.get("/internal/v1/model-profiles", response_model=list[ModelProfileResponse])
def list_model_profiles(
    user_id: str,
    request: Request,
    _: str = Depends(require_internal_client),
):
    payloads = _container(request).model_profile_service.list(user_id)
    return [ModelProfileResponse(**payload) for payload in payloads]


@router.get("/internal/v1/model-profiles/{profile_id}", response_model=ModelProfileResponse)
def get_model_profile(
    profile_id: str,
    user_id: str,
    request: Request,
    _: str = Depends(require_internal_client),
):
    payload = _container(request).model_profile_service.get(user_id, profile_id)
    if payload is None:
        return ModelProfileResponse(id=profile_id, user_id=user_id)
    return ModelProfileResponse(**payload)


@router.delete("/internal/v1/model-profiles/{profile_id}", response_model=ModelProfileStatusResponse)
def delete_model_profile(
    profile_id: str,
    req: ModelProfileDeleteRequest,
    request: Request,
    _: str = Depends(require_internal_client),
):
    deleted = _container(request).model_profile_service.delete(req.user_id, profile_id)
    return ModelProfileStatusResponse(
        status="ok" if deleted else "not_found",
        profile_id=profile_id,
    )


@router.get("/internal/v1/request-traces/{trace_id}", response_model=RequestTraceResponse)
def get_request_trace(
    trace_id: str,
    request: Request,
    _: str = Depends(require_internal_client),
):
    payload = _container(request).trace_repo.get(trace_id)
    if payload is None:
        return RequestTraceResponse(trace_id=trace_id, request_type="not_found")
    return RequestTraceResponse(**payload)


@router.get("/internal/v1/request-traces", response_model=list[RequestTraceResponse])
def list_request_traces(
    user_id: str,
    limit: int = 20,
    request_type: str = "",
    run_id: str = "",
    message_id: str = "",
    skill_id: str = "",
    model_profile_id: str = "",
    request: Request = None,
    _: str = Depends(require_internal_client),
):
    payloads = _container(request).trace_repo.list(
        user_id=user_id,
        limit=limit,
        request_type=request_type,
        run_id=run_id,
        message_id=message_id,
        skill_id=skill_id,
        model_profile_id=model_profile_id,
    )
    return [RequestTraceResponse(**payload) for payload in payloads]


@router.get("/internal/v1/eval/quality/suites", response_model=list[QualitySuiteResponse])
def list_quality_suites(
    request: Request,
    _: str = Depends(require_internal_client),
):
    payloads = _container(request).quality_eval_service.list_suites()
    return [QualitySuiteResponse(**payload) for payload in payloads]


@router.post("/internal/v1/eval/quality/run", response_model=QualityBenchmarkRunResponse)
def run_quality_suite(
    req: QualityBenchmarkRunRequest,
    request: Request,
    _: str = Depends(require_internal_client),
):
    container = _container(request)
    trace_id = str(uuid.uuid4())
    base_trace = {
        "id": trace_id,
        "trace_id": trace_id,
        "type": "quality_benchmark",
        "request_type": "quality_benchmark",
        "user_id": req.user_id,
        "query_text": req.suite_name,
        "mode": "benchmark",
        "model_profile_id": "",
        "created_at": datetime.now(timezone.utc).isoformat(),
    }
    container.trace_repo.create_trace(base_trace)
    try:
        payload = container.quality_eval_service.run_suite(
            user_id=req.user_id,
            suite_name=req.suite_name,
            cleanup=req.cleanup,
            scope_prefix=req.scope_prefix,
        )
    except Exception as exc:
        container.trace_repo.mark_trace_error(
            trace_id,
            code="BENCHMARK_RUN_ERROR",
            message=str(exc),
            updates=base_trace,
        )
        raise
    backend_snapshot = build_health_payload(container)
    container.trace_repo.update_trace_metrics(
        trace_id,
        **{
            **base_trace,
            "output_preview": f"benchmark {req.suite_name}: average_score={payload.get('average_score', 0):.2f}",
            "latency_retrieve_ms": 0,
            "latency_generate_ms": 0,
            "latency_total_ms": 0,
            "diagnostics": {
                "passed_count": payload.get("passed_count"),
                "failed_count": payload.get("failed_count"),
                "average_score": payload.get("average_score"),
                "kind_scores": payload.get("kind_scores"),
                "scope_ids": payload.get("scope_ids"),
                "backend_snapshot": backend_snapshot.model_dump(),
            },
        },
    )
    return QualityBenchmarkRunResponse(
        trace_id=trace_id,
        backend_snapshot=backend_snapshot,
        **payload,
    )


@router.post("/internal/v1/index/upsert-batch", response_model=IndexUpsertBatchResponse)
def index_upsert_batch(
    req: IndexUpsertBatchRequest,
    request: Request,
    _: str = Depends(require_internal_client),
):
    result = _container(request).indexing_service.upsert_documents(
        user_id=req.user_id,
        scope_id=req.scope_id,
        scope_name=req.name,
        scope_type=req.scope_type,
        provider=req.provider,
        sync_mode=req.sync_mode,
        run_async=req.run_async,
        documents=[item.model_dump() for item in req.documents],
    )
    return IndexUpsertBatchResponse(**result.__dict__)


@router.delete("/internal/v1/index/scopes/{user_id}/{scope_id}", response_model=DeleteScopeResponse)
@router.delete("/internal/knowledge/{user_id}/{scope_id}")
def delete_scope(user_id: str, scope_id: str, request: Request, _: str = Depends(require_internal_client)):
    _container(request).scope_service.delete_scope(user_id, scope_id)
    return DeleteScopeResponse(status="ok", scope_id=scope_id)


@router.delete("/internal/v1/index/scopes/{scope_id}", response_model=DeleteScopeResponse)
def delete_scope_v1_body(
    scope_id: str,
    req: DeleteScopeRequest,
    request: Request,
    _: str = Depends(require_internal_client),
):
    _container(request).scope_service.delete_scope(req.user_id, scope_id)
    return DeleteScopeResponse(status="ok", scope_id=scope_id)


@router.post("/internal/knowledge/upsert", response_model=LegacyKnowledgeUpsertResponse)
def legacy_knowledge_upsert(
    req: LegacyKnowledgeUpsertRequest,
    request: Request,
    _: str = Depends(require_internal_client),
):
    result = _container(request).indexing_service.upsert_documents(
        user_id=req.user_id,
        scope_id=req.connection_id,
        scope_name=req.connection_meta.name or req.connection_id,
        scope_type="knowledge_connection",
        provider="knowledge",
        sync_mode="replace",
        documents=[
            {
                "external_id": item.doc_id,
                "doc_id": item.doc_id,
                "title": item.title,
                "repo": item.repo,
                "doc_ref": item.doc_ref,
                "source_url": item.source_url,
                "updated_at": item.updated_at,
                "raw_body": item.raw_body,
            }
            for item in req.documents
        ],
    )
    return LegacyKnowledgeUpsertResponse(
        document_count=result.document_count,
        chunk_count=result.chunk_count,
        index_version=result.index_version,
        job_id=result.job_id,
        documents=result.documents,
    )


@router.post("/internal/knowledge/sync-source", response_model=LegacyKnowledgeUpsertResponse)
def legacy_sync_source(
    req: LegacyKnowledgeSourceSyncRequest,
    request: Request,
    _: str = Depends(require_internal_client),
):
    result = _container(request).indexing_service.sync_source(
        user_id=req.user_id,
        scope_id=req.connection_id,
        name=req.name,
        provider=req.provider,
        raw_payload=req.raw_payload,
    )
    return LegacyKnowledgeUpsertResponse(
        document_count=result.document_count,
        chunk_count=result.chunk_count,
        index_version=result.index_version,
        job_id=result.job_id,
        documents=result.documents,
    )


@router.post("/internal/v1/index/sync-source", response_model=IndexUpsertBatchResponse)
def sync_source_v1(
    req: IndexSyncSourceRequest,
    request: Request,
    _: str = Depends(require_internal_client),
):
    result = _container(request).indexing_service.sync_source(
        user_id=req.user_id,
        scope_id=req.scope_id,
        name=req.name,
        provider=req.provider,
        run_async=req.run_async,
        raw_payload=req.raw_payload,
    )
    return IndexUpsertBatchResponse(**result.__dict__)


@router.get("/internal/v1/index/jobs/{job_id}", response_model=IndexJobResponse)
def get_index_job(job_id: str, request: Request, _: str = Depends(require_internal_client)):
    payload = _container(request).index_job_repo.get(job_id)
    if payload is None:
        return IndexJobResponse(job_id=job_id, status="not_found", error="index job was not found")
    return IndexJobResponse(**payload)


@router.post("/internal/v1/retrieve", response_model=RetrieveResponse)
def retrieve(
    req: RetrieveRequest,
    request: Request,
    _: str = Depends(require_internal_client),
):
    trace_context = (req.trace.model_dump() if req.trace else {})
    trace_id = str(trace_context.get("trace_id") or "").strip() or str(uuid.uuid4())
    base_trace = {
        "id": trace_id,
        "trace_id": trace_id,
        "type": "retrieve",
        "request_type": "retrieve",
        "user_id": req.user_id,
        "run_id": str(trace_context.get("run_id") or ""),
        "message_id": str(trace_context.get("message_id") or ""),
        "skill_id": str(trace_context.get("skill_id") or ""),
        "scope_ids": req.scope_ids,
        "query_text": req.query,
        "model_profile_id": "",
        "created_at": datetime.now(timezone.utc).isoformat(),
    }
    _container(request).trace_repo.create_trace(base_trace)
    try:
        result = _container(request).retrieval_service.retrieve(
            user_id=req.user_id,
            scope_ids=req.scope_ids,
            query=req.query,
            top_k=req.top_k,
        )
    except Exception as exc:
        _container(request).trace_repo.mark_trace_error(
            trace_id,
            code="RETRIEVAL_ERROR",
            message=str(exc),
            updates=base_trace,
        )
        raise
    result.trace_id = trace_id
    _container(request).trace_repo.update_trace_metrics(
        trace_id,
        **{
            **base_trace,
            "retrieved_chunk_ids": result.diagnostics.fused_chunk_ids,
            "reranked_chunk_ids": result.diagnostics.reranked_chunk_ids,
            "used_chunk_ids": result.diagnostics.used_chunk_ids,
            "latency_retrieve_ms": result.diagnostics.latency_total_ms,
            "latency_generate_ms": 0,
            "latency_total_ms": result.diagnostics.latency_total_ms,
            "retrieval_backend": result.diagnostics.backend,
            "candidate_mode": result.diagnostics.candidate_mode,
            "output_preview": "",
        },
    )
    return RetrieveResponse(
        trace_id=trace_id,
        query=req.query,
        sources=result.sources(),
        metrics={
            "retrieve_ms": result.diagnostics.latency_total_ms,
            "generate_ms": 0,
            "total_ms": result.diagnostics.latency_total_ms,
        },
    )


@router.post("/internal/v1/evidence/merge", response_model=EvidenceMergeResponse)
def merge_evidence(
    req: EvidenceMergeRequest,
    request: Request,
    _: str = Depends(require_internal_client),
):
    result = _container(request).evidence_service.merge(
        user_id=req.user_id,
        goal=req.goal,
        mode=req.mode,
        evidences=req.evidences,
        skill_snapshot=req.skill_snapshot or {},
        top_k=req.top_k,
    )
    return EvidenceMergeResponse(
        trace_id=result.trace_id,
        goal=result.goal,
        mode=result.mode,
        evidences=[item.to_dict() for item in result.evidences],
        duplicate_count=result.duplicate_count,
        group_count=result.group_count,
        conflict_count=result.conflict_count,
        group_labels=result.group_labels,
        warnings=result.warnings,
        theme_summaries=result.theme_summaries,
        conflict_details=result.conflict_details,
        provider_summary=result.provider_summary,
    )


@router.post("/internal/v1/chat/stream")
def chat_stream_v1(
    req: ChatStreamRequest,
    request: Request,
    _: str = Depends(require_internal_client),
):
    payload = req.model_dump()
    if req.message_id:
        payload.setdefault("trace", {})
        payload["trace"]["message_id"] = req.message_id
    events = _container(request).chat_service.stream_chat(payload)
    return StreamingResponse(_sse(events), media_type="text/event-stream")


@router.post("/internal/chat/stream")
def chat_stream_legacy(
    req: LegacyChatRequest,
    request: Request,
    _: str = Depends(require_internal_client),
):
    payload = req.model_dump()
    payload["scope_ids"] = payload.pop("connection_ids", [])
    events = _container(request).chat_service.stream_chat(payload)
    return StreamingResponse(_sse(events), media_type="text/event-stream")


@router.post("/internal/v1/report/generate", response_model=ReportGenerateResponse)
def report_generate(
    req: ReportGenerateRequest,
    request: Request,
    _: str = Depends(require_internal_client),
):
    retrieval = RetrievalResult(query=req.goal, passages=[])
    if req.scope_ids:
        retrieval = _container(request).retrieval_service.retrieve(
            user_id=req.user_id,
            scope_ids=req.scope_ids,
            query=req.goal,
        )
    report = _container(request).report_service.generate(
        user_id=req.user_id,
        scope_ids=req.scope_ids,
        goal=req.goal,
        mode=req.mode,
        passages=retrieval.passages,
        evidences=req.evidences,
        evidence_diagnostics=req.evidence_diagnostics.model_dump() if req.evidence_diagnostics else None,
        retrieval_diagnostics=retrieval.diagnostics.to_dict() if req.scope_ids else None,
        skill_snapshot=req.skill_snapshot or {},
        model_profile=req.model_profile.model_dump() if req.model_profile else None,
        trace_context={
            **(req.trace.model_dump() if req.trace else {}),
            **({"run_id": req.run_id} if req.run_id else {}),
        }
        if (req.trace or req.run_id)
        else None,
        chat_model=req.chat_model,
        chat_api_base=req.chat_api_base,
        chat_api_key=req.chat_api_key,
    )
    return ReportGenerateResponse(
        trace_id=report.trace_id,
        summary=report.summary,
        outline_markdown=report.outline_markdown,
        draft_markdown=report.draft_markdown,
        retrieval_markdown=report.retrieval_markdown,
        report_markdown=report.report_markdown,
        outline_sections=[item.to_dict() for item in report.outline_sections],
        draft_sections=[item.to_dict() for item in report.draft_sections],
        report_sections=[item.to_dict() for item in report.report_sections],
        sources=report.sources,
        metrics=report.metrics,
    )
