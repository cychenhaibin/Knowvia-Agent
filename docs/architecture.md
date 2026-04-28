# Architecture

Knowvia is split into three layers:

1. `app/`: Expo client optimized around runs, timelines, and report delivery
2. `server/`: Go API, authentication, planner, worker orchestration, and tools
3. `infra/`: Postgres/pgvector and Redis for durable runs and background jobs

## Runtime Flow

1. User creates a run with a goal and optional mode override.
2. API stores the run, emits `run.created`, and enqueues execution.
3. Planner classifies the task into `kb_only`, `web_only`, or
   `hybrid_research`.
4. Executor walks explicit steps:
   - planning
   - yuque search
   - web search
   - page extract
   - evidence merge
   - report writer
   - finalize
5. Each step updates storage and streams SSE events.
6. Final report and sources are stored as artifacts for later retrieval.

## Design Notes

- The agent loop is explicit and deterministic, not a hidden ReAct loop.
- Knowledge sync and run execution are modeled as separate job classes.
- The server uses a shared store interface, with Postgres/pgvector as the
  durable default and memory mode retained as a fallback for isolated local
  debugging.
