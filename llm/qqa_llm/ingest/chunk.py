from __future__ import annotations

import hashlib
import re
from dataclasses import dataclass
from typing import List

from qqa_llm.core.config import Settings
from qqa_llm.domain.models import KnowledgeChunk, KnowledgeDocument
from qqa_llm.ingest.normalize import DocumentNormalizer


@dataclass
class ChunkBuilder:
    settings: Settings
    normalizer: DocumentNormalizer

    def build_chunks(self, document: KnowledgeDocument) -> List[KnowledgeChunk]:
        paragraphs = [part.strip() for part in document.normalized_body.split("\n") if part.strip()]
        if not paragraphs:
            return []

        chunks: List[KnowledgeChunk] = []
        current_parts: List[str] = []
        current_len = 0

        for paragraph in paragraphs:
            if current_parts and current_len + len(paragraph) > self.settings.chunk_size:
                chunks.append(self._make_chunk(document, len(chunks), "\n".join(current_parts)))
                overlap_text = self._tail("\n".join(current_parts), self.settings.chunk_overlap)
                current_parts = [overlap_text] if overlap_text else []
                current_len = len(overlap_text)

            current_parts.append(paragraph)
            current_len += len(paragraph)

        if current_parts:
            chunks.append(self._make_chunk(document, len(chunks), "\n".join(current_parts)))

        return chunks

    def _make_chunk(self, document: KnowledgeDocument, index: int, content: str) -> KnowledgeChunk:
        heading_path = self.normalizer.extract_heading_path(content)
        lexical_text = self.normalizer.build_lexical_text(
            title=document.title,
            repo=document.repo,
            heading_path=heading_path,
            content=content,
        )
        chunk_id = hashlib.sha256(f"{document.version_id}:{index}".encode("utf-8")).hexdigest()
        return KnowledgeChunk(
            id=chunk_id,
            scope_id=document.scope_id,
            user_id=document.user_id,
            document_id=document.id,
            version_id=document.version_id,
            provider=document.provider,
            repo=document.repo,
            title=document.title,
            heading_path=heading_path,
            source_url=document.source_url,
            updated_at=document.source_updated_at,
            chunk_index=index,
            content=content,
            lexical_text=lexical_text,
            token_count=self._count_tokens(content),
        )

    def _count_tokens(self, content: str) -> int:
        return len(re.findall(r"[A-Za-z0-9_]+|[\u4e00-\u9fff]", content or ""))

    def _tail(self, content: str, size: int) -> str:
        content = (content or "").strip()
        if len(content) <= size:
            return content
        return content[-size:]
