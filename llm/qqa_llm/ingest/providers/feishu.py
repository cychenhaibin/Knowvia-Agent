from __future__ import annotations

from datetime import datetime, timezone
from typing import Any, Dict, List

from qqa_llm.core.errors import ExternalProviderError


class FeishuPayloadParser:
    def parse(self, payload: Dict[str, Any], *, entry_type: str, entry_token: str) -> List[Dict[str, Any]]:
        if not isinstance(payload, dict):
            raise ExternalProviderError("feishu raw payload must be an object")

        metadata_map = payload.get("document_metadata") or {}
        raw_contents = payload.get("raw_contents") or {}
        if not isinstance(metadata_map, dict) or not isinstance(raw_contents, dict):
            raise ExternalProviderError("feishu raw payload is missing metadata or content maps")

        entry_type = (entry_type or payload.get("entry_type") or "docx").strip().lower() or "docx"
        if entry_type == "wiki_space":
            return self._parse_wiki_space(payload, metadata_map, raw_contents)

        doc_id = str(entry_token or payload.get("entry_token") or "").strip()
        if not doc_id:
            raise ExternalProviderError("feishu entry_token is required")
        raw_body = self._extract_raw_content(raw_contents.get(doc_id))
        if not raw_body:
            raise ExternalProviderError("feishu raw content is empty")
        metadata = metadata_map.get(doc_id) or {}
        return [
            self._build_document(
                document_id=doc_id,
                raw_body=raw_body,
                title=str(metadata.get("title") or doc_id).strip() or doc_id,
                updated_at=self._normalize_updated_at(str(metadata.get("update_time") or "").strip()),
            )
        ]

    def _parse_wiki_space(
        self,
        payload: Dict[str, Any],
        metadata_map: Dict[str, Any],
        raw_contents: Dict[str, Any],
    ) -> List[Dict[str, Any]]:
        documents: List[Dict[str, Any]] = []
        root_node = payload.get("root_node") or {}
        repo = str(root_node.get("title") or "飞书知识库").strip() or "飞书知识库"
        for node in payload.get("space_nodes") or []:
            if not isinstance(node, dict):
                continue
            obj_type = str(node.get("obj_type") or node.get("objType") or "").strip().lower()
            obj_token = str(node.get("obj_token") or node.get("objToken") or "").strip()
            if obj_type not in {"doc", "docx"} or not obj_token:
                continue
            raw_body = self._extract_raw_content(raw_contents.get(obj_token))
            if not raw_body:
                continue
            metadata = metadata_map.get(obj_token) or {}
            documents.append(
                self._build_document(
                    document_id=obj_token,
                    raw_body=raw_body,
                    title=str(node.get("title") or metadata.get("title") or obj_token).strip() or obj_token,
                    updated_at=self._normalize_updated_at(
                        str(metadata.get("update_time") or node.get("obj_edit_time") or "").strip()
                    ),
                    repo=repo,
                    doc_ref=str(node.get("node_token") or node.get("nodeToken") or obj_token).strip() or obj_token,
                    source_url=f"https://feishu.cn/wiki/{obj_token}",
                )
            )
        return documents

    def _build_document(
        self,
        *,
        document_id: str,
        raw_body: str,
        title: str,
        updated_at: str,
        repo: str = "飞书云文档",
        doc_ref: str = "",
        source_url: str = "",
    ) -> Dict[str, Any]:
        return {
            "external_id": document_id,
            "repo": repo,
            "title": title,
            "doc_ref": doc_ref or document_id,
            "source_url": source_url or f"https://feishu.cn/docx/{document_id}",
            "updated_at": updated_at,
            "raw_body": raw_body,
            "provider": "feishu",
        }

    def _extract_raw_content(self, payload: Any) -> str:
        if isinstance(payload, str):
            return payload.strip()
        if isinstance(payload, dict):
            return str(payload.get("content") or payload.get("raw_content") or payload.get("rawContent") or "").strip()
        return ""

    def _normalize_updated_at(self, raw: str) -> str:
        raw = (raw or "").strip()
        if not raw:
            return ""
        if raw.isdigit():
            timestamp = int(raw)
            if timestamp > 1_000_000_000_000:
                timestamp /= 1000
            return datetime.fromtimestamp(timestamp, tz=timezone.utc).isoformat()
        return raw
