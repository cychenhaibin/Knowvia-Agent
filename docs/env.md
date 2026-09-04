# Environment

## App

- `EXPO_PUBLIC_API_BASE_URL`

## Server

- `QQA_SERVER_ADDR`
- `QQA_ENV` (`development` explicitly enables local-only defaults)
- `QQA_STORE_BACKEND`
- `QQA_JWT_SECRET`
- `QQA_ACCESS_TTL_MINUTES`
- `QQA_REFRESH_TTL_HOURS`
- `QQA_QUEUE_MODE`
- `QQA_REDIS_ADDR`
- `QQA_POSTGRES_DSN`
- `QQA_OPENAI_BASE_URL`
- `QQA_OPENAI_API_KEY`
- `QQA_OPENAI_CHAT_MODEL`
- `QQA_OPENAI_EMBEDDING_MODEL`
- `QQA_RERANK_MODEL`
- `QQA_PYTHON_PROXY_BASE_URL`
- `QQA_PYTHON_PROXY_TOKEN`
- `QQA_DEV_USERS`

`QQA_DEV_USERS` uses a comma-separated `username:password[:displayName]`
format and is only accepted when `QQA_ENV=development`.

Production startup requires an explicit non-development `QQA_JWT_SECRET`.
The API and Python launchers bind to loopback by default; set
`QQA_SERVER_ADDR` / `QQA_LLM_HOST` explicitly when an external listener is
required. Python autoreload is opt-in with `QQA_LLM_RELOAD=true`.

`QQA_STORE_BACKEND` accepts `postgres` or `memory`. When the variable is
omitted, the server automatically uses `postgres` if `QQA_POSTGRES_DSN` is
set, otherwise it falls back to the in-memory store.

When `QQA_PYTHON_PROXY_BASE_URL` is set, the Go server delegates knowledge sync
and forwarded knowledge chat to `quickque-agent/llm`, which normally runs at
`http://127.0.0.1:8000`. In that mode, start the Python service with
`quickque-agent/llm/run_server.sh` before triggering "立即同步" or knowledge chat.
Go -> Python internal calls use `QQA_PYTHON_PROXY_TOKEN` only.
