# GitHub Repo Analysis Task Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the GitHub repository analysis task flow that creates a task conversation, auto-sends a generated prompt, analyzes a public repository, and stores a Markdown Code Wiki artifact.

**Architecture:** Go detects GitHub task goals, owns Run/session state, scans repositories safely, and persists artifacts. The existing chat stream delegates task sessions to a Run task executor, while the LLM is used only for synthesis when available.

**Tech Stack:** Go API, PostgreSQL migrations, in-memory store, React Native Expo app, existing chat SSE flow, existing Run artifact storage.

---

### Task 1: Domain And Store Behavior

**Files:**
- Modify: `server/internal/domain/chat.go`
- Modify: `server/internal/domain/run.go`
- Modify: `server/internal/adapters/store/memory_chat_sessions.go`
- Modify: `server/internal/adapters/store/contract_chat_test.go`
- Create: `server/migrations/000023_github_repo_analysis_tasks.sql`

- [ ] Add `ChatSessionKind`, `RunKind`, new task/session fields, and `ArtifactKindCodeWiki`.
- [ ] Write a store contract assertion that task chat sessions are excluded from `ListChatSessions`.
- [ ] Implement memory and Postgres filtering for normal chat sessions.

### Task 2: Run Creation Creates Task Conversation

**Files:**
- Modify: `server/internal/run/create_input.go`
- Modify: `server/internal/run/service.go`
- Modify: `server/internal/run/ports.go`
- Modify: `server/internal/run/service_test.go`
- Modify: `server/internal/adapters/httpapi/api_run_dto.go`
- Modify: `server/internal/adapters/httpapi/run_handler.go`

- [ ] Write a failing test that a GitHub URL goal creates a `github_repo_analysis` Run, `taskSessionId`, and generated prompt.
- [ ] Add GitHub URL extraction and task prompt generation.
- [ ] Return the new Run fields from the API.

### Task 3: Repository Analysis Executor

**Files:**
- Create: `server/internal/gitrepo/analyzer.go`
- Create: `server/internal/gitrepo/analyzer_test.go`
- Create: `server/internal/run/task_conversation.go`
- Modify: `server/internal/run/service.go`
- Modify: `server/internal/run/service_test.go`

- [ ] Write tests using temporary local repositories for file filtering, tree generation, and key file selection.
- [ ] Implement safe clone/scan/analyze with size limits and deterministic fallback Markdown.
- [ ] Save `code_wiki` artifacts and update Run status from task conversation execution.

### Task 4: Chat Stream Delegates Task Sessions

**Files:**
- Modify: `server/internal/chat/ports.go`
- Modify: `server/internal/chat/service.go`
- Modify: `server/internal/chat/conversation.go`
- Modify: `server/internal/app/chat.go`
- Modify: `server/internal/app/run.go`

- [ ] Write a chat service test proving task sessions delegate to the Run task executor.
- [ ] Add a task conversation runner interface and wire Run service into chat service.
- [ ] Preserve normal chat behavior for ordinary sessions.

### Task 5: App Auto-Navigation And Auto-Send

**Files:**
- Modify: `app/types/api.ts`
- Modify: `app/lib/api.ts`
- Modify: `app/modules/chat/screens/ComposeScreen.tsx`
- Modify: `app/modules/chat/screens/RunsScreen.tsx`
- Modify: `app/modules/chat/screens/RunHistoryScreen.tsx`

- [ ] Route GitHub task creation success to the chat tab with `taskSessionId`, `taskPrompt`, and `runId`.
- [ ] Auto-send the task prompt once after navigation.
- [ ] Keep task sessions out of the normal session drawer through backend filtering.

### Task 6: Technical Documentation

**Files:**
- Modify: `docs/llm-technical-design.md`
- Modify: `docs/go-python-rag-architecture.md`
- Modify: `README.zh-CN.md`

- [ ] Document the Go/LLM responsibility split.
- [ ] Document the new interaction flow and data fields.
- [ ] Document current MVP limits: public GitHub repositories only, read-only analysis, no repository code execution.
