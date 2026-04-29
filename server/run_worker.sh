#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")"

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

echo "🚀 Starting Knowvia worker..."
echo "   queue mode: ${QQA_QUEUE_MODE:-inline}"
echo "   redis: ${QQA_REDIS_ADDR:-<unset>}"
echo "   queue: ${QQA_QUEUE_NAME:-knowvia:tasks}"
echo "   workers: ${QQA_QUEUE_WORKERS:-4}"
echo "   store: ${QQA_STORE_BACKEND:-auto}"
echo ""

exec go run ./cmd/worker
