from __future__ import annotations

from typing import Any, Dict, List

from qqa_llm.core.errors import ExternalProviderError


class YuquePayloadParser:
    def parse(self, payload: Dict[str, Any]) -> List[Dict[str, Any]]:
        if not isinstance(payload, dict):
            raise ExternalProviderError("yuque raw payload must be an object")

        docs_by_namespace = payload.get("docs_by_namespace") or {}
        bodies_by_namespace = payload.get("bodies_by_namespace") or {}
        repos = payload.get("repos") or []
        documents: List[Dict[str, Any]] = []

        namespaces = [str(repo.get("namespace") or "").strip() for repo in repos if isinstance(repo, dict)]
        if not namespaces:
            namespaces = [str(key).strip() for key in docs_by_namespace.keys() if str(key).strip()]

        for namespace in namespaces:
            metas = docs_by_namespace.get(namespace) or []
            body_map = bodies_by_namespace.get(namespace) or {}
            if not isinstance(metas, list) or not isinstance(body_map, dict):
                continue
            for meta in metas:
                if not isinstance(meta, dict):
                    continue
                slug = str(meta.get("slug") or "").strip()
                if not slug:
                    continue
                raw_body = str(body_map.get(slug) or "").strip()
                if not raw_body:
                    continue
                documents.append(
                    {
                        "external_id": str(meta.get("id") or slug).strip(),
                        "repo": namespace,
                        "title": str(meta.get("title") or slug).strip(),
                        "doc_ref": slug,
                        "source_url": f"https://www.yuque.com/{namespace}/{slug}",
                        "updated_at": str(meta.get("updated_at") or "").strip(),
                        "raw_body": raw_body,
                        "provider": "yuque",
                    }
                )
        return documents
