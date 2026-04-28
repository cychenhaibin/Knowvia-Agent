from __future__ import annotations

from dataclasses import asdict
from datetime import datetime, timezone
from time import perf_counter
import re
import uuid
from typing import Dict, Generator, Iterable, List, Set, Tuple

from qqa_llm.core.config import Settings
from qqa_llm.inference.generator import build_generation_client
from qqa_llm.prompt.chat_builder import ChatPromptBuilder
from qqa_llm.services.model_profile_service import ModelProfileService
from qqa_llm.services.retrieval_service import RetrievalService
from qqa_llm.services.skill_service import SkillService
from qqa_llm.storage.trace_repo import TraceRepository


class ChatService:
    def __init__(
        self,
        settings: Settings,
        *,
        retrieval_service: RetrievalService,
        skill_service: SkillService,
        model_profile_service: ModelProfileService,
        trace_repo: TraceRepository,
        prompt_builder: ChatPromptBuilder,
    ) -> None:
        self.settings = settings
        self.retrieval_service = retrieval_service
        self.skill_service = skill_service
        self.model_profile_service = model_profile_service
        self.trace_repo = trace_repo
        self.prompt_builder = prompt_builder

    def stream_chat(self, payload: Dict[str, object]) -> Generator[Dict[str, object], None, None]:
        user_id = str(payload.get("user_id") or "").strip()
        message = str(payload.get("message") or "").strip()
        scope_ids = [str(item).strip() for item in (payload.get("scope_ids") or payload.get("connection_ids") or []) if str(item).strip()]
        if not user_id or not message:
            yield {
                "type": "error",
                "error": {
                    "code": "VALIDATION_ERROR",
                    "message": "user_id and message are required",
                    "retryable": False,
                },
            }
            return
        trace_context = dict(payload.get("trace") or {})
        trace_id = str(trace_context.get("trace_id") or "").strip() or str(uuid.uuid4())
        mode, skill_prompt = self.skill_service.resolve(
            user_id=user_id,
            skill_id=str(payload.get("skill_id") or "").strip(),
            requested_mode=str(payload.get("mode") or "").strip(),
            requested_prompt=str(payload.get("skill_prompt") or "").strip(),
            skill_snapshot=payload.get("skill_snapshot") or {},
        )
        model_profile = self.model_profile_service.resolve(
            user_id=user_id,
            purpose="chat_fast",
            requested_profile=payload.get("model_profile") or {},
            legacy_model=str(payload.get("chat_model") or "").strip(),
            legacy_api_base=str(payload.get("chat_api_base") or "").strip(),
            legacy_api_key=str(payload.get("chat_api_key") or "").strip(),
        )
        base_trace = {
            "id": trace_id,
            "trace_id": trace_id,
            "type": "chat",
            "request_type": "chat",
            "user_id": user_id,
            "run_id": str(trace_context.get("run_id") or ""),
            "message_id": str(trace_context.get("message_id") or ""),
            "skill_id": str(trace_context.get("skill_id") or payload.get("skill_id") or ""),
            "scope_ids": scope_ids,
            "query_text": message,
            "question": message,
            "mode": mode,
            "model_profile_id": model_profile.id,
            "created_at": datetime.now(timezone.utc).isoformat(),
        }
        self.trace_repo.create_trace(base_trace)
        # Retrieval happens once up front so the client sees a stable source
        # list before the generation stream starts.
        retrieval = self.retrieval_service.retrieve(
            user_id=user_id,
            scope_ids=scope_ids,
            query=message,
        )
        retrieval.trace_id = trace_id
        yield {
            "type": "retrieval",
            "traceId": trace_id,
            "sources": retrieval.sources(),
            "metrics": {
                "retrieveMs": retrieval.diagnostics.latency_total_ms,
                "generateMs": 0,
                "totalMs": retrieval.diagnostics.latency_total_ms,
            },
        }

        prompt = self.prompt_builder.build(
            question=message,
            mode=mode,
            skill_prompt=skill_prompt,
            passages=retrieval.passages,
        )
        generator = build_generation_client(self.settings, profile=model_profile)
        answer_parts = []
        generation_started = perf_counter()
        try:
            for chunk in generator.generate_stream(prompt):
                if not chunk:
                    continue
                answer_parts.append(chunk)
                yield {"type": "chunk", "traceId": trace_id, "content": chunk}
        except Exception as exc:
            self.trace_repo.mark_trace_error(
                trace_id,
                code="GENERATION_ERROR",
                message=str(exc),
                updates={
                    **base_trace,
                    "retrieved_chunk_ids": retrieval.diagnostics.fused_chunk_ids,
                    "reranked_chunk_ids": retrieval.diagnostics.reranked_chunk_ids,
                    "used_chunk_ids": retrieval.diagnostics.used_chunk_ids,
                    "latency_retrieve_ms": retrieval.diagnostics.latency_total_ms,
                    "latency_generate_ms": int((perf_counter() - generation_started) * 1000),
                    "latency_total_ms": retrieval.diagnostics.latency_total_ms + int((perf_counter() - generation_started) * 1000),
                    "retrieval_backend": retrieval.diagnostics.backend,
                    "candidate_mode": retrieval.diagnostics.candidate_mode,
                },
            )
            yield {
                "type": "error",
                "traceId": trace_id,
                "error": {
                    "code": "GENERATION_ERROR",
                    "message": str(exc),
                    "retryable": False,
                },
            }
            return

        answer = "".join(answer_parts).strip()
        generation_elapsed_ms = int((perf_counter() - generation_started) * 1000)
        grounded_sources = self._select_grounded_sources(answer, retrieval)
        grounded_chunk_ids = [str(item.get("chunk_id") or "") for item in grounded_sources if str(item.get("chunk_id") or "")]
        self.trace_repo.update_trace_metrics(
            trace_id,
            **{
                **base_trace,
                "output_preview": answer[:400],
                "source_count": len(grounded_sources),
                "retrieved_chunk_ids": retrieval.diagnostics.fused_chunk_ids,
                "reranked_chunk_ids": retrieval.diagnostics.reranked_chunk_ids,
                "used_chunk_ids": grounded_chunk_ids,
                "latency_retrieve_ms": retrieval.diagnostics.latency_total_ms,
                "latency_generate_ms": generation_elapsed_ms,
                "latency_total_ms": retrieval.diagnostics.latency_total_ms + generation_elapsed_ms,
                "retrieval_backend": retrieval.diagnostics.backend,
                "candidate_mode": retrieval.diagnostics.candidate_mode,
            },
        )
        yield {
            "type": "done",
            "traceId": trace_id,
            "content": answer,
            "answer": answer,
            # The retrieval event can expose a broader shortlist while the
            # final chat answer should only keep sources that can be grounded
            # back to specific answer lines.
            "sources": grounded_sources,
            "metrics": {
                "retrieveMs": retrieval.diagnostics.latency_total_ms,
                "generateMs": generation_elapsed_ms,
                "totalMs": retrieval.diagnostics.latency_total_ms + generation_elapsed_ms,
            },
        }

    def _select_grounded_sources(self, answer: str, retrieval) -> List[Dict[str, object]]:
        passages = list(retrieval.passages or [])
        if not passages:
            return []

        answer_lines = self._meaningful_answer_lines(answer)
        if not answer_lines:
            return []

        grounded: Dict[str, Dict[str, object]] = {}
        passage_keys: Dict[str, Tuple[str, str]] = {}
        for line in answer_lines:
            best = self._best_passage_for_line(line, passages)
            if best is None:
                continue
            passage, anchor_score, overlap_score = best
            if anchor_score <= 0 or overlap_score < 2:
                continue
            key = passage.chunk_id or f"{passage.title}|{passage.url}"
            if key not in grounded:
                payload = passage.to_source_dict()
                payload["matched_answer_lines"] = []
                payload["match_score"] = 0
                grounded[key] = payload
                passage_keys[key] = (passage.title.strip().lower(), passage.url.strip().lower())
            grounded[key]["matched_answer_lines"].append(line)
            grounded[key]["match_score"] = max(int(grounded[key]["match_score"]), anchor_score * 10 + overlap_score)

        deduped: List[Dict[str, object]] = []
        seen_titles: Set[Tuple[str, str]] = set()
        items = list(grounded.items())
        items.sort(key=lambda item: int(item[1].get("match_score") or 0), reverse=True)
        for key, payload in items:
            dedupe_key = passage_keys[key]
            if dedupe_key in seen_titles:
                continue
            seen_titles.add(dedupe_key)
            deduped.append(payload)
        return deduped

    def _feature_tokens(self, text: str) -> Set[str]:
        raw = (text or "").strip().lower()
        if not raw:
            return set()
        tokens: Set[str] = set()
        for word in re.findall(r"[a-z0-9_]{2,}", raw):
            tokens.add(word)
        for chunk in re.findall(r"[\u4e00-\u9fff]+", raw):
            if len(chunk) <= 2:
                tokens.add(chunk)
                continue
            for size in (2, 3):
                if len(chunk) < size:
                    continue
                for idx in range(len(chunk) - size + 1):
                    tokens.add(chunk[idx : idx + size])
        return tokens

    def _meaningful_answer_lines(self, answer: str) -> List[str]:
        lines: List[str] = []
        for raw_line in (answer or "").splitlines():
            line = raw_line.strip()
            if not line:
                continue
            line = re.sub(r"^[\-\*\d\.\)\s]+", "", line).strip()
            if len(line) < 6:
                continue
            lines.append(line)
        return lines

    def _best_passage_for_line(self, line: str, passages) -> Tuple[object, int, int] | None:
        if not line or not passages:
            return None
        best: Tuple[object, int, int, float] | None = None
        line_features = self._feature_tokens(line)
        if not line_features:
            return None
        for passage in passages:
            passage_text = f"{passage.title}\n{passage.snippet}\n{passage.content}".lower()
            overlap = len(line_features & self._feature_tokens(passage_text))
            anchor = self._line_anchor_score(line.lower(), passage_text)
            score = float(passage.score)
            candidate = (passage, anchor, overlap, score)
            if best is None or (anchor, overlap, score) > (best[1], best[2], best[3]):
                best = candidate
        if best is None:
            return None
        return best[0], best[1], best[2]

    def _best_line_anchor(self, answer_lines: List[str], passage) -> Tuple[int, int]:
        if not answer_lines:
            return 0, 0
        passage_text = f"{passage.title}\n{passage.snippet}\n{passage.content}".lower()
        best_overlap = 0
        best_anchor = 0
        for line in answer_lines:
            overlap = len(self._feature_tokens(line) & self._feature_tokens(passage_text))
            anchor = self._line_anchor_score(line.lower(), passage_text)
            if (anchor, overlap) > (best_anchor, best_overlap):
                best_anchor = anchor
                best_overlap = overlap
        return best_overlap, best_anchor

    def _line_anchor_score(self, line: str, passage_text: str) -> int:
        chunks = re.findall(r"[\u4e00-\u9fff]{4,}|[a-z0-9_]{4,}", line)
        if not chunks:
            return 0
        score = 0
        for chunk in chunks:
            if chunk in passage_text:
                score += 1
        return score
