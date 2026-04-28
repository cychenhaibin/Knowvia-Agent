from __future__ import annotations

from dataclasses import asdict, dataclass, field
from typing import Any, Dict, List


@dataclass
class ScopeRef:
    user_id: str
    scope_id: str
    scope_type: str
    scope_external_id: str
    provider: str
    name: str
    status: str = "active"
    latest_index_version: str = ""
    document_count: int = 0
    chunk_count: int = 0
    last_indexed_at: str = ""
    created_at: str = ""
    updated_at: str = ""

    def to_dict(self) -> Dict[str, Any]:
        return asdict(self)


@dataclass
class SkillMirror:
    id: str
    user_id: str
    slug: str
    title: str
    scope: str = "global"
    definition_id: str = ""
    revision_id: str = ""
    kind: str = "chat_profile"
    description: str = ""
    prompt: str = ""
    mode: str = "answer"
    source: str = "manual"
    enabled: bool = True
    repo_url: str = ""
    runtime_spec: Dict[str, Any] = field(default_factory=dict)
    created_at: str = ""
    updated_at: str = ""

    def to_dict(self) -> Dict[str, Any]:
        return asdict(self)


@dataclass
class KnowledgeDocument:
    id: str
    scope_id: str
    user_id: str
    provider: str
    external_id: str
    repo: str
    title: str
    doc_ref: str
    source_url: str
    source_updated_at: str
    content_hash: str
    version_id: str
    raw_body: str
    normalized_body: str

    def to_dict(self) -> Dict[str, Any]:
        return asdict(self)


@dataclass
class KnowledgeChunk:
    id: str
    scope_id: str
    user_id: str
    document_id: str
    version_id: str
    provider: str
    repo: str
    title: str
    heading_path: str
    source_url: str
    updated_at: str
    chunk_index: int
    content: str
    lexical_text: str
    token_count: int = 0

    def to_dict(self) -> Dict[str, Any]:
        return asdict(self)


@dataclass
class RetrievedPassage:
    chunk_id: str
    scope_id: str
    connection_id: str
    document_id: str
    provider: str
    title: str
    url: str
    repo: str
    snippet: str
    content: str
    score: float

    def to_source_dict(self) -> Dict[str, Any]:
        return {
            "type": "knowledge_base",
            "provider": self.provider,
            "scope_id": self.scope_id,
            "connection_id": self.connection_id,
            "document_id": self.document_id,
            "chunk_id": self.chunk_id,
            "title": self.title,
            "url": self.url,
            "repo": self.repo,
            "snippet": self.snippet,
            "score": self.score,
        }


@dataclass
class ModelProfile:
    id: str
    user_id: str = ""
    purpose: str = "chat_fast"
    provider: str = "fallback"
    name: str = ""
    base_url: str = ""
    api_key_ref: str = ""
    model_name: str = ""
    temperature: float = 0.2
    max_tokens: int = 4096
    is_default: bool = False
    updated_at: str = ""

    def to_dict(self) -> Dict[str, Any]:
        return asdict(self)


@dataclass
class ResolvedModelProfile:
    id: str
    purpose: str
    provider: str
    name: str
    base_url: str
    api_key: str
    model_name: str
    temperature: float = 0.2
    max_tokens: int = 4096

    def to_trace_dict(self) -> Dict[str, Any]:
        return {
            "id": self.id,
            "purpose": self.purpose,
            "provider": self.provider,
            "name": self.name,
            "model_name": self.model_name,
        }


@dataclass
class TraceContext:
    trace_id: str = ""
    run_id: str = ""
    message_id: str = ""
    skill_id: str = ""

    def to_dict(self) -> Dict[str, Any]:
        return asdict(self)


@dataclass
class EvidenceRecord:
    provider: str
    connection_id: str
    document_id: str
    chunk_id: str
    title: str
    repo: str
    url: str
    snippet: str
    body: str
    score: float
    authority_score: float = 0.0
    theme_label: str = ""

    def to_dict(self) -> Dict[str, Any]:
        return asdict(self)


@dataclass
class RetrievalDiagnostics:
    backend: str = "file"
    candidate_mode: str = "python_full_scan"
    lexical_candidate_ids: List[str] = field(default_factory=list)
    vector_candidate_ids: List[str] = field(default_factory=list)
    fused_chunk_ids: List[str] = field(default_factory=list)
    reranked_chunk_ids: List[str] = field(default_factory=list)
    used_chunk_ids: List[str] = field(default_factory=list)
    chunk_catalog: Dict[str, Dict[str, str]] = field(default_factory=dict)
    latency_lexical_ms: int = 0
    latency_vector_ms: int = 0
    latency_fusion_ms: int = 0
    latency_rerank_ms: int = 0
    latency_total_ms: int = 0

    def to_dict(self) -> Dict[str, Any]:
        return asdict(self)


@dataclass
class RetrievalResult:
    query: str
    passages: List[RetrievedPassage] = field(default_factory=list)
    diagnostics: RetrievalDiagnostics = field(default_factory=RetrievalDiagnostics)
    trace_id: str = ""

    def sources(self) -> List[Dict[str, Any]]:
        return [passage.to_source_dict() for passage in self.passages]


@dataclass
class ChatPrompt:
    system_prompt: str
    user_prompt: str
    mode: str
    question: str
    skill_prompt: str
    passages: List[RetrievedPassage] = field(default_factory=list)


@dataclass
class ReportPrompt:
    system_prompt: str
    user_prompt: str
    goal: str
    mode: str
    passages: List[RetrievedPassage] = field(default_factory=list)
    diagnostics: "ReportEvidenceDiagnostics" = field(default_factory=lambda: ReportEvidenceDiagnostics())
    outline_markdown: str = ""
    draft_markdown: str = ""


@dataclass
class IndexingResult:
    document_count: int
    chunk_count: int
    index_version: str
    job_id: str = ""
    documents: List[Dict[str, Any]] = field(default_factory=list)
    changed_documents: Dict[str, int] = field(default_factory=dict)
    status: str = ""
    execution_mode: str = "sync"


@dataclass
class GeneratedReport:
    summary: str
    report_markdown: str
    outline_markdown: str = ""
    draft_markdown: str = ""
    retrieval_markdown: str = ""
    outline_sections: List["ReportSection"] = field(default_factory=list)
    draft_sections: List["ReportSection"] = field(default_factory=list)
    report_sections: List["ReportSection"] = field(default_factory=list)
    sources: List[Dict[str, Any]] = field(default_factory=list)
    trace_id: str = ""
    metrics: Dict[str, int] = field(default_factory=dict)


@dataclass
class ReportSection:
    key: str
    title: str
    content_markdown: str
    heading_level: int = 2

    def to_dict(self) -> Dict[str, Any]:
        return asdict(self)


@dataclass
class ReportEvidenceDiagnostics:
    duplicate_count: int = 0
    group_count: int = 0
    conflict_count: int = 0
    group_labels: List[str] = field(default_factory=list)
    warnings: List[str] = field(default_factory=list)
    theme_summaries: List[Dict[str, Any]] = field(default_factory=list)
    conflict_details: List[Dict[str, Any]] = field(default_factory=list)
    provider_summary: Dict[str, int] = field(default_factory=dict)


@dataclass
class EvidenceMergeResult:
    goal: str
    mode: str
    trace_id: str = ""
    evidences: List[EvidenceRecord] = field(default_factory=list)
    duplicate_count: int = 0
    group_count: int = 0
    conflict_count: int = 0
    group_labels: List[str] = field(default_factory=list)
    warnings: List[str] = field(default_factory=list)
    theme_summaries: List[Dict[str, Any]] = field(default_factory=list)
    conflict_details: List[Dict[str, Any]] = field(default_factory=list)
    provider_summary: Dict[str, int] = field(default_factory=dict)
