from __future__ import annotations

from collections import defaultdict
from typing import Dict, List, Sequence


def reciprocal_rank_fusion(rankings: Sequence[List[Dict[str, object]]], k: int = 60) -> List[Dict[str, object]]:
    scores = defaultdict(float)
    for ranking in rankings:
        for index, item in enumerate(ranking):
            chunk_id = str(item.get("chunk_id") or "")
            if not chunk_id:
                continue
            scores[chunk_id] += 1.0 / (k + index + 1)
    merged = [{"chunk_id": chunk_id, "score": score} for chunk_id, score in scores.items()]
    merged.sort(key=lambda item: float(item["score"]), reverse=True)
    return merged
