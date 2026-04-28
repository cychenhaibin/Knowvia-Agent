#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")"

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

echo "🚀 Starting Knowvia server..."
echo "   addr: ${QQA_SERVER_ADDR:-0.0.0.0:8088}"
echo "   store: ${QQA_STORE_BACKEND:-auto}"
echo "   postgres: ${QQA_POSTGRES_DSN:-<unset>}"
echo ""

exec go run ./cmd/api
