from __future__ import annotations

import secrets
from typing import Optional

from fastapi import Header, HTTPException, Request


def require_internal_client(request: Request, authorization: Optional[str] = Header(default=None)) -> str:
    if not authorization:
        raise HTTPException(status_code=401, detail="missing internal authorization")
    parts = authorization.split()
    if len(parts) != 2 or parts[0].lower() != "bearer":
        raise HTTPException(status_code=401, detail="invalid internal authorization format")
    token = parts[1].strip()
    settings = getattr(request.app.state, "settings", None)
    expected = getattr(settings, "internal_auth_token", "")
    if not expected:
        raise HTTPException(status_code=500, detail="internal auth token is not configured")
    if not secrets.compare_digest(token, expected):
        raise HTTPException(status_code=401, detail="invalid internal auth token")
    return "go-proxy"
