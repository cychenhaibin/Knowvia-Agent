from __future__ import annotations

from typing import Any, Dict, List, Optional

from pydantic import AliasChoices, BaseModel, Field


class HealthResponse(BaseModel):
    status: str
    message: str
    checks: Dict[str, str] = Field(default_factory=dict)
    degraded_reasons: List[str] = Field(default_factory=list, validation_alias=AliasChoices("degraded_reasons", "degradedReasons"))
    scope_count: int = 0
    index_backend: str = "file"
    generator_backend: str = "fallback"
    db: Dict[str, Any] = Field(default_factory=dict)
    generator: Dict[str, Any] = Field(default_factory=dict)
    embedding: Dict[str, Any] = Field(default_factory=dict)
    rerank: Dict[str, Any] = Field(default_factory=dict)
    model_profiles: Dict[str, Dict[str, Any]] = Field(default_factory=dict)


class SkillMirrorRequest(BaseModel):
    id: str = Field(validation_alias=AliasChoices("id", "skillId"))
    user_id: str = Field(validation_alias=AliasChoices("user_id", "userId"))
    slug: str
    title: str
    scope: str = "global"
    definition_id: str = Field(default="", validation_alias=AliasChoices("definition_id", "definitionId"))
    revision_id: str = Field(default="", validation_alias=AliasChoices("revision_id", "revisionId"))
    kind: str = "chat_profile"
    description: str = ""
    prompt: str = ""
    mode: str = "answer"
    source: str = "manual"
    enabled: bool = True
    repo_url: str = Field(default="", validation_alias=AliasChoices("repo_url", "repoUrl"))
    runtime_spec: Dict[str, Any] = Field(default_factory=dict, validation_alias=AliasChoices("runtime_spec", "runtimeSpec"))
    created_at: str = Field(default="", validation_alias=AliasChoices("created_at", "createdAt"))
    updated_at: str = Field(default="", validation_alias=AliasChoices("updated_at", "updatedAt"))


class SkillStatusResponse(BaseModel):
    status: str = "ok"
    skill_id: str = ""


class ConnectionMetaRequest(BaseModel):
    id: str
    name: str
    group_login: str = Field(default="", validation_alias=AliasChoices("group_login", "groupLogin"))
    namespace: str = ""


class LegacyKnowledgeDocumentRequest(BaseModel):
    doc_id: str = Field(validation_alias=AliasChoices("doc_id", "docId"))
    title: str
    repo: str
    doc_ref: str = Field(validation_alias=AliasChoices("doc_ref", "docRef"))
    source_url: str = Field(default="", validation_alias=AliasChoices("source_url", "sourceUrl"))
    updated_at: str = Field(default="", validation_alias=AliasChoices("updated_at", "updatedAt"))
    raw_body: str


class SyncedKnowledgeDocumentResponse(BaseModel):
    doc_id: str
    title: str
    repo: str
    doc_ref: str
    source_url: str = ""
    updated_at: str = ""
    chunk_count: int = 0


class LegacyKnowledgeUpsertRequest(BaseModel):
    user_id: str = Field(validation_alias=AliasChoices("user_id", "userId"))
    connection_id: str = Field(validation_alias=AliasChoices("connection_id", "connectionId"))
    connection_meta: ConnectionMetaRequest = Field(validation_alias=AliasChoices("connection_meta", "connectionMeta"))
    documents: List[LegacyKnowledgeDocumentRequest] = Field(default_factory=list)


class LegacyKnowledgeUpsertResponse(BaseModel):
    document_count: int
    chunk_count: int
    index_version: str
    job_id: str = ""
    documents: List[SyncedKnowledgeDocumentResponse] = Field(default_factory=list)


class LegacyKnowledgeSourceSyncRequest(BaseModel):
    user_id: str = Field(validation_alias=AliasChoices("user_id", "userId"))
    connection_id: str = Field(validation_alias=AliasChoices("connection_id", "connectionId"))
    name: str
    provider: str = "yuque"
    raw_payload: Dict[str, Any] = Field(default_factory=dict, validation_alias=AliasChoices("raw_payload", "rawPayload"))


class LegacyChatRequest(BaseModel):
    user_id: str
    message: str
    connection_ids: List[str] = Field(default_factory=list)
    chat_model: str = ""
    chat_api_base: str = ""
    chat_api_key: str = ""
    enable_search: bool = Field(default=False, validation_alias=AliasChoices("enable_search", "enableSearch"))
    temperature: Optional[float] = None
    skill_id: str = ""
    skill_prompt: str = ""
    mode: str = "answer"
    skill_snapshot: Optional[Dict[str, Any]] = None


class RetrievalSource(BaseModel):
    type: str = "knowledge_base"
    provider: str = ""
    scope_id: str = ""
    connection_id: str = ""
    document_id: str = ""
    chunk_id: str = ""
    title: str
    url: str = ""
    repo: str = ""
    snippet: str = ""
    score: float = 0.0


class ModelProfileRequest(BaseModel):
    profile_id: str = Field(default="", validation_alias=AliasChoices("profile_id", "profileId"))
    purpose: str = ""
    provider: str = ""
    name: str = ""
    base_url: str = Field(default="", validation_alias=AliasChoices("base_url", "baseUrl"))
    api_key_ref: str = Field(default="", validation_alias=AliasChoices("api_key_ref", "apiKeyRef"))
    api_key: str = Field(default="", validation_alias=AliasChoices("api_key", "apiKey"))
    model_name: str = Field(default="", validation_alias=AliasChoices("model_name", "modelName"))
    temperature: float = 0.2
    max_tokens: int = Field(default=4096, validation_alias=AliasChoices("max_tokens", "maxTokens"))
    is_default: bool = Field(default=False, validation_alias=AliasChoices("is_default", "isDefault"))
    updated_at: str = Field(default="", validation_alias=AliasChoices("updated_at", "updatedAt"))


class ModelProfileStatusResponse(BaseModel):
    status: str = "ok"
    profile_id: str = Field(default="", validation_alias=AliasChoices("profile_id", "profileId"))


class ModelProfileDeleteRequest(BaseModel):
    user_id: str = Field(validation_alias=AliasChoices("user_id", "userId"))


class SkillDeleteRequest(BaseModel):
    user_id: str = Field(validation_alias=AliasChoices("user_id", "userId"))


class ModelProfileResponse(BaseModel):
    id: str
    user_id: str = Field(default="", validation_alias=AliasChoices("user_id", "userId"))
    purpose: str = ""
    provider: str = ""
    name: str = ""
    base_url: str = Field(default="", validation_alias=AliasChoices("base_url", "baseUrl"))
    api_key_ref: str = Field(default="", validation_alias=AliasChoices("api_key_ref", "apiKeyRef"))
    model_name: str = Field(default="", validation_alias=AliasChoices("model_name", "modelName"))
    temperature: float = 0.2
    max_tokens: int = Field(default=4096, validation_alias=AliasChoices("max_tokens", "maxTokens"))
    is_default: bool = Field(default=False, validation_alias=AliasChoices("is_default", "isDefault"))
    updated_at: str = Field(default="", validation_alias=AliasChoices("updated_at", "updatedAt"))


class ModelProfileUpsertRequest(ModelProfileRequest):
    user_id: str = Field(validation_alias=AliasChoices("user_id", "userId"))


class TraceContextRequest(BaseModel):
    trace_id: str = Field(default="", validation_alias=AliasChoices("trace_id", "traceId"))
    run_id: str = Field(default="", validation_alias=AliasChoices("run_id", "runId"))
    message_id: str = Field(default="", validation_alias=AliasChoices("message_id", "messageId"))
    skill_id: str = Field(default="", validation_alias=AliasChoices("skill_id", "skillId"))


class MetricsResponse(BaseModel):
    retrieve_ms: int = 0
    generate_ms: int = 0
    total_ms: int = 0


class MergedEvidence(BaseModel):
    provider: str = ""
    connection_id: str = ""
    document_id: str = ""
    chunk_id: str = ""
    title: str
    repo: str = ""
    url: str = ""
    snippet: str = ""
    body: str = ""
    score: float = 0.0
    authority_score: float = Field(default=0.0, validation_alias=AliasChoices("authority_score", "authorityScore"))
    theme_label: str = Field(default="", validation_alias=AliasChoices("theme_label", "themeLabel"))


class EvidenceThemeSummary(BaseModel):
    label: str
    evidence_count: int = Field(default=0, validation_alias=AliasChoices("evidence_count", "evidenceCount"))
    provider_summary: Dict[str, int] = Field(default_factory=dict, validation_alias=AliasChoices("provider_summary", "providerSummary"))
    top_titles: List[str] = Field(default_factory=list, validation_alias=AliasChoices("top_titles", "topTitles"))
    avg_score: float = Field(default=0.0, validation_alias=AliasChoices("avg_score", "avgScore"))
    avg_authority_score: float = Field(default=0.0, validation_alias=AliasChoices("avg_authority_score", "avgAuthorityScore"))


class EvidenceConflictDetail(BaseModel):
    label: str
    variant_count: int = Field(default=0, validation_alias=AliasChoices("variant_count", "variantCount"))
    titles: List[str] = Field(default_factory=list)
    providers: List[str] = Field(default_factory=list)
    recommendation: str = ""


class IndexDocumentInput(BaseModel):
    external_id: str = Field(validation_alias=AliasChoices("external_id", "externalId"))
    title: str
    repo: str
    doc_ref: str = Field(validation_alias=AliasChoices("doc_ref", "docRef"))
    source_url: str = Field(default="", validation_alias=AliasChoices("source_url", "sourceUrl"))
    updated_at: str = Field(
        default="",
        validation_alias=AliasChoices("updated_at", "updatedAt", "source_updated_at", "sourceUpdatedAt"),
    )
    raw_body: str = Field(validation_alias=AliasChoices("raw_body", "rawBody"))
    normalized_body: str = Field(default="", validation_alias=AliasChoices("normalized_body", "normalizedBody"))
    metadata: Dict[str, Any] = Field(default_factory=dict)
    provider: str = ""


class IndexUpsertBatchRequest(BaseModel):
    user_id: str = Field(validation_alias=AliasChoices("user_id", "userId"))
    scope_id: str = Field(validation_alias=AliasChoices("scope_id", "scopeId"))
    name: str
    scope_type: str = Field(default="knowledge_connection", validation_alias=AliasChoices("scope_type", "scopeType"))
    provider: str = "knowledge"
    sync_mode: str = Field(default="replace", validation_alias=AliasChoices("sync_mode", "syncMode"))
    run_async: bool = Field(default=False, validation_alias=AliasChoices("run_async", "runAsync"))
    documents: List[IndexDocumentInput] = Field(default_factory=list)


class ChangedDocumentsResponse(BaseModel):
    added: int = 0
    updated: int = 0
    deleted: int = 0
    unchanged: int = 0


class IndexUpsertBatchResponse(BaseModel):
    document_count: int
    chunk_count: int
    index_version: str
    job_id: str = ""
    status: str = ""
    execution_mode: str = Field(default="sync", validation_alias=AliasChoices("execution_mode", "executionMode"))
    documents: List[SyncedKnowledgeDocumentResponse] = Field(default_factory=list)
    changed_documents: ChangedDocumentsResponse = Field(
        default_factory=ChangedDocumentsResponse,
        validation_alias=AliasChoices("changed_documents", "changedDocuments"),
    )


class DeleteScopeRequest(BaseModel):
    user_id: str = Field(validation_alias=AliasChoices("user_id", "userId"))


class DeleteScopeResponse(BaseModel):
    status: str = "ok"
    scope_id: str = ""


class IndexSyncSourceRequest(BaseModel):
    user_id: str = Field(validation_alias=AliasChoices("user_id", "userId"))
    scope_id: str = Field(validation_alias=AliasChoices("scope_id", "scopeId"))
    name: str
    provider: str = "yuque"
    run_async: bool = Field(default=False, validation_alias=AliasChoices("run_async", "runAsync"))
    raw_payload: Dict[str, Any] = Field(default_factory=dict, validation_alias=AliasChoices("raw_payload", "rawPayload"))


class RetrieveRequest(BaseModel):
    user_id: str = Field(validation_alias=AliasChoices("user_id", "userId"))
    scope_ids: List[str] = Field(default_factory=list, validation_alias=AliasChoices("scope_ids", "scopeIds"))
    query: str
    top_k: int = Field(default=0, validation_alias=AliasChoices("top_k", "topK"))
    return_trace: bool = Field(default=True, validation_alias=AliasChoices("return_trace", "returnTrace"))
    trace: Optional[TraceContextRequest] = None


class RetrieveResponse(BaseModel):
    trace_id: str = ""
    query: str
    sources: List[RetrievalSource] = Field(default_factory=list)
    metrics: MetricsResponse = Field(default_factory=MetricsResponse)


class EvidenceMergeRequest(BaseModel):
    user_id: str = Field(validation_alias=AliasChoices("user_id", "userId"))
    goal: str
    mode: str = "auto"
    skill_snapshot: Optional[Dict[str, Any]] = Field(default=None, validation_alias=AliasChoices("skill_snapshot", "skillSnapshot"))
    evidences: List[Dict[str, Any]] = Field(default_factory=list)
    top_k: int = Field(default=0, validation_alias=AliasChoices("top_k", "topK"))


class EvidenceMergeResponse(BaseModel):
    trace_id: str = Field(default="", validation_alias=AliasChoices("trace_id", "traceId"))
    goal: str
    mode: str
    evidences: List[MergedEvidence] = Field(default_factory=list)
    duplicate_count: int = 0
    group_count: int = 0
    conflict_count: int = 0
    group_labels: List[str] = Field(default_factory=list)
    warnings: List[str] = Field(default_factory=list)
    theme_summaries: List[EvidenceThemeSummary] = Field(default_factory=list, validation_alias=AliasChoices("theme_summaries", "themeSummaries"))
    conflict_details: List[EvidenceConflictDetail] = Field(default_factory=list, validation_alias=AliasChoices("conflict_details", "conflictDetails"))
    provider_summary: Dict[str, int] = Field(default_factory=dict, validation_alias=AliasChoices("provider_summary", "providerSummary"))


class EvidenceDiagnostics(BaseModel):
    duplicate_count: int = 0
    group_count: int = 0
    conflict_count: int = 0
    group_labels: List[str] = Field(default_factory=list)
    warnings: List[str] = Field(default_factory=list)
    theme_summaries: List[EvidenceThemeSummary] = Field(default_factory=list, validation_alias=AliasChoices("theme_summaries", "themeSummaries"))
    conflict_details: List[EvidenceConflictDetail] = Field(default_factory=list, validation_alias=AliasChoices("conflict_details", "conflictDetails"))
    provider_summary: Dict[str, int] = Field(default_factory=dict, validation_alias=AliasChoices("provider_summary", "providerSummary"))


class RequestTraceResponse(BaseModel):
    trace_id: str = Field(default="", validation_alias=AliasChoices("trace_id", "traceId"))
    request_type: str = ""
    user_id: str = Field(default="", validation_alias=AliasChoices("user_id", "userId"))
    run_id: str = Field(default="", validation_alias=AliasChoices("run_id", "runId"))
    message_id: str = Field(default="", validation_alias=AliasChoices("message_id", "messageId"))
    skill_id: str = Field(default="", validation_alias=AliasChoices("skill_id", "skillId"))
    model_profile_id: str = Field(default="", validation_alias=AliasChoices("model_profile_id", "modelProfileId"))
    scope_ids: List[str] = Field(default_factory=list, validation_alias=AliasChoices("scope_ids", "scopeIds"))
    query_text: str = Field(default="", validation_alias=AliasChoices("query_text", "queryText"))
    mode: str = ""
    retrieved_chunk_ids: List[str] = Field(default_factory=list, validation_alias=AliasChoices("retrieved_chunk_ids", "retrievedChunkIds"))
    reranked_chunk_ids: List[str] = Field(default_factory=list, validation_alias=AliasChoices("reranked_chunk_ids", "rerankedChunkIds"))
    used_chunk_ids: List[str] = Field(default_factory=list, validation_alias=AliasChoices("used_chunk_ids", "usedChunkIds"))
    latency_retrieve_ms: int = Field(default=0, validation_alias=AliasChoices("latency_retrieve_ms", "latencyRetrieveMs"))
    latency_generate_ms: int = Field(default=0, validation_alias=AliasChoices("latency_generate_ms", "latencyGenerateMs"))
    latency_total_ms: int = Field(default=0, validation_alias=AliasChoices("latency_total_ms", "latencyTotalMs"))
    retrieval_backend: str = Field(default="", validation_alias=AliasChoices("retrieval_backend", "retrievalBackend"))
    candidate_mode: str = Field(default="", validation_alias=AliasChoices("candidate_mode", "candidateMode"))
    output_preview: str = Field(default="", validation_alias=AliasChoices("output_preview", "outputPreview"))
    outline_sections: List["ReportSectionResponse"] = Field(default_factory=list, validation_alias=AliasChoices("outline_sections", "outlineSections"))
    draft_sections: List["ReportSectionResponse"] = Field(default_factory=list, validation_alias=AliasChoices("draft_sections", "draftSections"))
    report_sections: List["ReportSectionResponse"] = Field(default_factory=list, validation_alias=AliasChoices("report_sections", "reportSections"))
    error_code: str = Field(default="", validation_alias=AliasChoices("error_code", "errorCode"))
    error_message: str = Field(default="", validation_alias=AliasChoices("error_message", "errorMessage"))
    created_at: str = Field(default="", validation_alias=AliasChoices("created_at", "createdAt"))


class IndexJobResponse(BaseModel):
    job_id: str
    operation: str = ""
    job_type: str = ""
    status: str = ""
    user_id: str = ""
    scope_id: str = ""
    scope_name: str = ""
    scope_type: str = ""
    provider: str = ""
    document_count: int = 0
    chunk_count: int = 0
    index_version: str = ""
    started_at: str = ""
    finished_at: str = ""
    created_at: str = ""
    updated_at: str = ""
    error_code: str = ""
    error_message: str = ""
    error: str = ""


class ChatStreamRequest(BaseModel):
    user_id: str = Field(validation_alias=AliasChoices("user_id", "userId"))
    message: str
    scope_ids: List[str] = Field(default_factory=list, validation_alias=AliasChoices("scope_ids", "scopeIds"))
    chat_model: str = Field(default="", validation_alias=AliasChoices("chat_model", "chatModel"))
    chat_api_base: str = Field(default="", validation_alias=AliasChoices("chat_api_base", "chatApiBase"))
    chat_api_key: str = Field(default="", validation_alias=AliasChoices("chat_api_key", "chatApiKey"))
    enable_search: bool = Field(default=False, validation_alias=AliasChoices("enable_search", "enableSearch"))
    temperature: Optional[float] = None
    skill_id: str = Field(default="", validation_alias=AliasChoices("skill_id", "skillId"))
    skill_prompt: str = Field(default="", validation_alias=AliasChoices("skill_prompt", "skillPrompt"))
    mode: str = "answer"
    skill_snapshot: Optional[Dict[str, Any]] = Field(default=None, validation_alias=AliasChoices("skill_snapshot", "skillSnapshot", "skill_context", "skillContext"))
    model_profile: Optional[ModelProfileRequest] = Field(default=None, validation_alias=AliasChoices("model_profile", "modelProfile"))
    trace: Optional[TraceContextRequest] = None
    message_id: str = Field(default="", validation_alias=AliasChoices("message_id", "messageId"))


class ReportGenerateRequest(BaseModel):
    user_id: str = Field(validation_alias=AliasChoices("user_id", "userId"))
    goal: str
    scope_ids: List[str] = Field(default_factory=list, validation_alias=AliasChoices("scope_ids", "scopeIds"))
    run_id: str = Field(default="", validation_alias=AliasChoices("run_id", "runId"))
    mode: str = "report"
    chat_model: str = Field(default="", validation_alias=AliasChoices("chat_model", "chatModel"))
    chat_api_base: str = Field(default="", validation_alias=AliasChoices("chat_api_base", "chatApiBase"))
    chat_api_key: str = Field(default="", validation_alias=AliasChoices("chat_api_key", "chatApiKey"))
    skill_snapshot: Optional[Dict[str, Any]] = Field(default=None, validation_alias=AliasChoices("skill_snapshot", "skillSnapshot"))
    evidences: List[Dict[str, Any]] = Field(default_factory=list)
    evidence_diagnostics: Optional[EvidenceDiagnostics] = Field(default=None, validation_alias=AliasChoices("evidence_diagnostics", "evidenceDiagnostics"))
    model_profile: Optional[ModelProfileRequest] = Field(default=None, validation_alias=AliasChoices("model_profile", "modelProfile"))
    trace: Optional[TraceContextRequest] = None


class ReportSectionResponse(BaseModel):
    key: str
    title: str
    content_markdown: str = Field(default="", validation_alias=AliasChoices("content_markdown", "contentMarkdown"))
    heading_level: int = Field(default=2, validation_alias=AliasChoices("heading_level", "headingLevel"))


class ReportGenerateResponse(BaseModel):
    trace_id: str = ""
    summary: str
    outline_markdown: str = ""
    draft_markdown: str = ""
    retrieval_markdown: str = ""
    report_markdown: str
    outline_sections: List[ReportSectionResponse] = Field(default_factory=list, validation_alias=AliasChoices("outline_sections", "outlineSections"))
    draft_sections: List[ReportSectionResponse] = Field(default_factory=list, validation_alias=AliasChoices("draft_sections", "draftSections"))
    report_sections: List[ReportSectionResponse] = Field(default_factory=list, validation_alias=AliasChoices("report_sections", "reportSections"))
    sources: List[RetrievalSource] = Field(default_factory=list)
    metrics: MetricsResponse = Field(default_factory=MetricsResponse)


class QualitySuiteResponse(BaseModel):
    suite_name: str = Field(validation_alias=AliasChoices("suite_name", "suiteName"))
    description: str = ""
    scope_count: int = Field(default=0, validation_alias=AliasChoices("scope_count", "scopeCount"))
    retrieval_case_count: int = Field(default=0, validation_alias=AliasChoices("retrieval_case_count", "retrievalCaseCount"))
    evidence_case_count: int = Field(default=0, validation_alias=AliasChoices("evidence_case_count", "evidenceCaseCount"))
    report_case_count: int = Field(default=0, validation_alias=AliasChoices("report_case_count", "reportCaseCount"))
    retrieval_case_names: List[str] = Field(default_factory=list, validation_alias=AliasChoices("retrieval_case_names", "retrievalCaseNames"))
    evidence_case_names: List[str] = Field(default_factory=list, validation_alias=AliasChoices("evidence_case_names", "evidenceCaseNames"))
    report_case_names: List[str] = Field(default_factory=list, validation_alias=AliasChoices("report_case_names", "reportCaseNames"))


class QualityBenchmarkRunRequest(BaseModel):
    user_id: str = Field(validation_alias=AliasChoices("user_id", "userId"))
    suite_name: str = Field(default="builtin", validation_alias=AliasChoices("suite_name", "suiteName"))
    cleanup: bool = True
    scope_prefix: str = Field(default="", validation_alias=AliasChoices("scope_prefix", "scopePrefix"))


class QualityAssertionResponse(BaseModel):
    passed: bool
    message: str


class QualityMetricResponse(BaseModel):
    name: str
    score: float
    weight: float
    detail: str = ""


class QualityCaseResultResponse(BaseModel):
    name: str
    kind: str
    passed: bool
    score: float = 0.0
    trace_id: str = Field(default="", validation_alias=AliasChoices("trace_id", "traceId"))
    assertions: List[QualityAssertionResponse] = Field(default_factory=list)
    metrics: List[QualityMetricResponse] = Field(default_factory=list)


class QualityBenchmarkRunResponse(BaseModel):
    trace_id: str = Field(default="", validation_alias=AliasChoices("trace_id", "traceId"))
    suite_name: str = Field(validation_alias=AliasChoices("suite_name", "suiteName"))
    passed_count: int = Field(default=0, validation_alias=AliasChoices("passed_count", "passedCount"))
    failed_count: int = Field(default=0, validation_alias=AliasChoices("failed_count", "failedCount"))
    average_score: float = Field(default=0.0, validation_alias=AliasChoices("average_score", "averageScore"))
    kind_scores: Dict[str, float] = Field(default_factory=dict, validation_alias=AliasChoices("kind_scores", "kindScores"))
    markdown_report: str = Field(default="", validation_alias=AliasChoices("markdown_report", "markdownReport"))
    scope_ids: List[str] = Field(default_factory=list, validation_alias=AliasChoices("scope_ids", "scopeIds"))
    cleanup_applied: bool = Field(default=False, validation_alias=AliasChoices("cleanup_applied", "cleanupApplied"))
    backend_snapshot: Optional[HealthResponse] = Field(default=None, validation_alias=AliasChoices("backend_snapshot", "backendSnapshot"))
    results: List[QualityCaseResultResponse] = Field(default_factory=list)


RequestTraceResponse.model_rebuild()
