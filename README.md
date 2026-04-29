# Knowvia

**English** | [简体中文](README.zh-CN.md)

Knowvia is an AI research and delivery workspace for document knowledge bases. It treats every source as a knowledge connection, indexes the content, retrieves grounded evidence, and uses LLMs to produce answers, analysis, and structured reports.

## Overview

Knowvia is more than a chat interface. A user can start a run with a research goal, attach knowledge scopes and skills, then let the system plan, retrieve evidence, merge sources, generate reports, and deliver artifacts with traceable execution history.

## Core Features

- Knowledge connections: sync external documents into a unified knowledge index.
- Hybrid retrieval: combine lexical search, vector search, fusion, and reranking.
- Grounded chat: answer with evidence from selected knowledge scopes.
- Research runs: execute long-running tasks with plans, steps, sources, and artifacts.
- Structured reports: produce summaries, markdown reports, sources, and intermediate artifacts.
- Skill runtime: define response modes, tool permissions, and report styles through skills.
- Multi-client stack: Expo mobile app, Go API server, Python LLM engine, and standalone web site.

## Architecture

```text
app / web
   |
   v
server
   |
   +--> Postgres / Redis
   |
   v
llm
   |
   +--> Postgres + pgvector or file backend
   +--> OpenAI-compatible models / Ollama / local fallbacks
```

| Directory | Description |
| --- | --- |
| `app/` | Expo React Native mobile client for auth, chat, runs, knowledge selection, and skills. |
| `web/` | Standalone marketing or product web site. |
| `server/` | Go API and orchestration layer for auth, sessions, knowledge connections, skills, runs, events, and background execution. |
| `llm/` | Python Evidence Reasoning Engine for indexing, retrieval, reranking, grounded chat, and report generation. |
| `infra/` | Local Postgres/pgvector and Redis infrastructure. |
| `docs/` | Architecture notes, event protocol, and LLM database/API design docs. |

## Runtime Flow

1. The user enters a goal in the app and selects knowledge scopes and a skill.
2. The Go server creates a run and stores intent, mode, and context.
3. The run service plans and executes retrieval, search, evidence merging, and report generation.
4. The LLM service handles indexing, hybrid retrieval, reranking, grounded chat, and report writing.
5. The app displays run progress, sources, artifacts, and the final report through APIs and events.

## Prerequisites

- Node.js 20.x
- Go 1.25+
- Python 3.10+
- Docker and Docker Compose
- Optional: Ollama or an OpenAI-compatible model provider

## Local Development

All commands below assume you are starting from the repository root.

### 1. Start Infra

```bash
cd infra
docker compose up -d
```

This starts local Postgres/pgvector and Redis. You can also use your own database by setting the DSN values in `server/.env` and `llm/.env`.

### 2. Start Go Server

```bash
cd server
cp .env.example .env
./run_server.sh
```

The server address is controlled by `QQA_SERVER_ADDR`, usually `0.0.0.0:8088`. On startup, the server loads `server/.env` and applies the database schema when using Postgres.

If you want background runs and knowledge sync jobs to execute in a separate worker process, set:

```bash
QQA_QUEUE_MODE=redis
```

then start the worker too:

```bash
cd server
./run_worker.sh
```

### 3. Start LLM Engine

```bash
cd llm
cp .env.example .env
python -m venv venv
./venv/bin/pip install -r requirements.txt
./run_server.sh
```

The LLM service listens on `http://127.0.0.1:8000` by default. If `QQA_PYTHON_PROXY_BASE_URL` is enabled in `server/.env`, the Go server forwards indexing, grounded chat, and report generation to this service.

### 4. Start Mobile App

```bash
cd app
nvm use
npm install
npm run dev
```

Common commands:

```bash
npm run dev
npm run dev:android
npm run android
npm run ios
npm run web
```

### 5. Start Web Site

```bash
cd web
npm install
npm run dev
```

The web site runs at `http://127.0.0.1:4174` by default.

## Android Packaging

```bash
cd app
nvm use
npm install
npm run build:android:release
```

Available build commands:

```bash
npm run build:android
npm run build:android:debug
npm run build:android:release
npm run build:android:aab
```

Android artifacts are written to `app/dist/android/`.

## Configuration

### Server Environment

Main variables in `server/.env`:

| Variable | Description |
| --- | --- |
| `QQA_SERVER_ADDR` | Go API listen address. |
| `QQA_STORE_BACKEND` | Store backend, usually `postgres` or `memory`. |
| `QQA_POSTGRES_DSN` | Main database DSN for the Go server. |
| `QQA_REDIS_ADDR` | Redis address for queueing and async features. |
| `QQA_QUEUE_MODE` | `inline` to execute tasks inside the API process, `redis` to dispatch to a separate worker. |
| `QQA_QUEUE_NAME` | Redis queue name prefix, default `knowvia:tasks`. |
| `QQA_QUEUE_WORKERS` | Redis worker concurrency. |
| `QQA_QUEUE_MAX_ATTEMPTS` | Max retry attempts before a task moves to the failed queue. |
| `QQA_QUEUE_RETRY_BASE_SECONDS` | Base retry backoff in seconds; retries use exponential backoff. |
| `QQA_QUEUE_RETRY_MAX_SECONDS` | Max retry backoff in seconds. |
| `QQA_QUEUE_METRICS_LOG_SECONDS` | Periodic worker metrics log interval. |
| `QQA_QUEUE_SHUTDOWN_TIMEOUT_SECONDS` | Graceful worker shutdown timeout. |
| `QQA_QUEUE_DEDUP_TTL_SECONDS` | Idempotency key retention in seconds for queue deduplication. |
| `QQA_OPENAI_BASE_URL` | OpenAI-compatible API base URL. |
| `QQA_OPENAI_API_KEY` | Model provider API key. |
| `QQA_PYTHON_PROXY_BASE_URL` | Python LLM service URL. |
| `QQA_PYTHON_PROXY_TOKEN` | Internal auth token from Go to Python. |
| `QQA_DEV_USERS` | Local development users. |

### LLM Environment

Main variables in `llm/.env`:

| Variable | Description |
| --- | --- |
| `QQA_INDEX_BACKEND` | Index backend: `auto`, `postgres`, or `file`. |
| `QQA_POSTGRES_DSN` | Database DSN for LLM indexing. |
| `QQA_POSTGRES_SCHEMA` | Postgres schema used by the LLM engine. |
| `QQA_GENERATOR_BACKEND` | Generation backend: OpenAI-compatible, Ollama, or fallback. |
| `QQA_EMBEDDING_BACKEND` | Embedding backend. |
| `QQA_RERANK_BACKEND` | Reranking backend. |
| `OPENAI_API_BASE` | OpenAI-compatible service URL. |
| `OPENAI_API_KEY` | OpenAI-compatible API key. |
| `OLLAMA_BASE_URL` | Ollama service URL. |
| `OLLAMA_MODEL` | Ollama model name. |

## Testing

### Go

```bash
cd server
go test ./...
```

### LLM

```bash
cd llm
./venv/bin/python -m unittest tests.test_internal_api tests.test_backend_selection
```

Some local environments may need x86_64 execution if Python dependencies were installed for that architecture:

```bash
cd llm
arch -x86_64 ./venv/bin/python -m unittest tests.test_internal_api tests.test_backend_selection
```

### Web

```bash
cd web
npm run build
```

## Documentation

- `docs/events.md`: run event stream protocol.
- `docs/go-python-rag-architecture.md`: Go gateway and Python RAG architecture notes.
- `docs/llm-technical-design.md`: LLM technical design.
- `docs/llm-database-and-api-design.md`: LLM database and API design.
- `docs/llm-module-and-class-design.md`: LLM module and class design.

## Development Notes

- Go is the authority for user-facing state and orchestration: auth, sessions, knowledge connections, skills, and run lifecycle.
- Python LLM owns reasoning capabilities: indexing, retrieval, reranking, generation, and quality evaluation.
- The app displays state through APIs and event streams; it does not call the LLM service directly.
- Postgres + pgvector is the recommended index backend; the file backend is useful for local debugging and lightweight tests.
- Skills are managed by the Go store and mirrored into the LLM runtime.
