# FluxA Model Groups Diagnostics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Safely identify the FluxA upstream failure that prevents model groups from loading on device.

**Architecture:** The FluxA model fetcher keeps its public API behavior while recording a credential-free failure summary at the upstream boundary. The summary has only route, status, and JSON-envelope classification, enabling a compatible parser or endpoint correction after device reproduction.

**Tech Stack:** Go, net/http, Go testing, Android Expo development client.

## Global Constraints

- Never log or return FluxA usernames, passwords, bearer tokens, request headers, or raw response bodies.
- Preserve `ErrFluxAReauthenticationRequired` for HTTP 401/403 and `ErrFluxAUnavailable` for other upstream failures.
- Keep upstream timeout and redirect behavior unchanged.

---

### Task 1: Record safe upstream response diagnostics

**Files:**
- Modify: `server/internal/auth/fluxa_models.go:116-164`
- Modify: `server/internal/auth/fluxa_models_test.go`

**Interfaces:**
- Consumes: `fluxAModelGroupsFetcher.get(context.Context, string, string, string) (json.RawMessage, error)`.
- Produces: a standard-library log line with `fluxa_model_groups_upstream_failure`, route, status, and envelope classification.

- [ ] **Step 1: Write the failing test**

Add a test server returning `502` from `/api/models`, invoke `List`, and assert captured logger output contains `route=/api/models` and `status=502` but not `upstream-token` or the response-body marker `do-not-log-me`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd server && go test ./internal/auth -run TestFluxAModelGroupsLogsSafeUpstreamFailure -count=1`

Expected: FAIL because no diagnostic is emitted.

- [ ] **Step 3: Write minimal implementation**

In `get`, classify only `transport_error`, `http_status`, `invalid_json`, `unsuccessful_envelope`, and `empty_data`; log route/status/classification before the existing safe error return. Do not include request data or response body values.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd server && go test ./internal/auth -run TestFluxAModelGroupsLogsSafeUpstreamFailure -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

Run: `git add server/internal/auth/fluxa_models.go server/internal/auth/fluxa_models_test.go && git commit -m 'fix: log safe FluxA model fetch diagnostics'`

### Task 2: Reproduce and apply the compatible upstream fix

**Files:**
- Modify: `server/internal/auth/fluxa_models.go` only if the captured route/status/schema requires it.
- Modify: `server/internal/auth/fluxa_models_test.go`

**Interfaces:**
- Consumes: safe diagnostic from Task 1 and the existing `parseFluxAModels` and `parseFluxAAccountGroups` functions.
- Produces: normalized `[]FluxAModelGroup` for FluxA's observed successful payload.

- [ ] **Step 1: Write the failing test**

Encode the exact credential-free account and model response shape observed after device reproduction in `TestListFluxAModelGroups...`, asserting expected group names and model IDs.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd server && go test ./internal/auth -run TestListFluxAModelGroups -count=1`

Expected: FAIL because the parser does not accept the observed response shape.

- [ ] **Step 3: Write minimal implementation**

Extend only the parser branch required by the observed successful payload; preserve current accepted array and `{models: [...]}` forms and group normalization.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd server && go test ./internal/auth -run TestListFluxAModelGroups -count=1`

Expected: PASS.

- [ ] **Step 5: Verify on device and commit**

Restart `go run ./cmd/api` with the same local configuration, sign in to FluxA again because the local store is in-memory, open Model Groups, verify groups and models render, run `cd server && go test ./...`, and commit the parser fix.
