#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")"

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

echo "🚀 Starting Knowvia LLM..."
echo "   backend: ${QQA_INDEX_BACKEND:-auto}"
echo "   postgres: ${QQA_POSTGRES_DSN:-<unset>}"
echo ""

PYTHON_BIN=./venv/bin/python
LLM_HOST="${QQA_LLM_HOST:-127.0.0.1}"
UVICORN_ARGS=(--host "$LLM_HOST" --port "${QQA_LLM_PORT:-8000}")
if [ "${QQA_LLM_RELOAD:-false}" = "true" ]; then
  UVICORN_ARGS+=(--reload)
fi
if [ "$(uname -m)" = "arm64" ]; then
  exec arch -x86_64 "$PYTHON_BIN" -m uvicorn qqa_llm.main:app "${UVICORN_ARGS[@]}"
fi

exec "$PYTHON_BIN" -m uvicorn qqa_llm.main:app "${UVICORN_ARGS[@]}"
