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
if [ "$(uname -m)" = "arm64" ]; then
  exec arch -x86_64 "$PYTHON_BIN" -m uvicorn qqa_llm.main:app --host 0.0.0.0 --port 8000 --reload
fi

exec "$PYTHON_BIN" -m uvicorn qqa_llm.main:app --host 0.0.0.0 --port 8000 --reload
