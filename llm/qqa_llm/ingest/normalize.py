from __future__ import annotations

import html
import json
import re
from datetime import datetime
from typing import Any, Dict, List


HTML_TAG_RE = re.compile(
    r"""</?
        [a-zA-Z][\w\-]*
        (?:\s+[^<>]*?)?
        /?>
    """,
    re.VERBOSE,
)

HEADING_RE = re.compile(r"^(#{1,6})\s+(.+)$")


class DocumentNormalizer:
    def clean_text(self, text: str) -> str:
        text = HTML_TAG_RE.sub("\n", text)
        text = html.unescape(text)
        text = text.replace("\r\n", "\n").replace("\r", "\n").replace("\t", " ")
        text = re.sub(r"[ \t\f\v]+", " ", text)
        lines = [re.sub(r"\s+", " ", line).strip() for line in text.split("\n")]
        return "\n".join(line for line in lines if line)

    def normalize_body(self, raw_body: str) -> str:
        raw_body = (raw_body or "").strip()
        if not raw_body:
            return ""
        if raw_body.startswith("{") or raw_body.startswith("["):
            structured = self._normalize_structured_body(raw_body)
            if structured:
                return structured
        return self.clean_text(raw_body)

    def build_lexical_text(self, *, title: str, repo: str, heading_path: str, content: str) -> str:
        parts = [title.strip(), repo.strip(), heading_path.strip(), content.strip()]
        return "\n".join(part for part in parts if part)

    def normalize_updated_at(self, raw: str) -> str:
        raw = (raw or "").strip()
        if not raw:
            return ""
        for fmt in ("%Y-%m-%dT%H:%M:%S.%f%z", "%Y-%m-%dT%H:%M:%S%z", "%Y-%m-%d"):
            try:
                return datetime.strptime(raw, fmt).isoformat()
            except ValueError:
                continue
        return raw

    def extract_heading_path(self, text: str) -> str:
        headings: List[str] = []
        for line in (text or "").splitlines():
            match = HEADING_RE.match(line.strip())
            if match:
                headings.append(match.group(2).strip())
        return " > ".join(headings[:3])

    def _normalize_structured_body(self, text: str) -> str:
        try:
            payload = json.loads(text)
        except json.JSONDecodeError:
            return ""
        if isinstance(payload, dict) and payload.get("format") == "laketable":
            return self._normalize_lake_table(payload)
        return ""

    def _normalize_lake_table(self, payload: Dict[str, Any]) -> str:
        sheets = payload.get("sheet") or []
        if not sheets:
            return ""
        sheet = sheets[0] or {}
        if not isinstance(sheet, dict):
            return ""

        column_names: List[str] = []
        lines: List[str] = []
        for column in sheet.get("columns") or []:
            if not isinstance(column, dict):
                continue
            name = str(column.get("name") or "").strip()
            if name:
                column_names.append(name)
        if column_names:
            lines.append("表格列: " + ", ".join(column_names))

        for view in (sheet.get("views") or {}).values():
            if not isinstance(view, dict):
                continue
            for group in view.get("groupData") or []:
                if not isinstance(group, dict):
                    continue
                rows = group.get("rows") or []
                if not rows:
                    continue
                label = str(group.get("titleValue") or "未分组").strip()
                lines.append(f"{label}: {len(rows)} 条")

        return self.clean_text("\n".join(lines))
