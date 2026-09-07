# Task 1 Report

Status: complete

Commit: `132db86` (`feat: proxy FluxA current balance`)

Implemented:
- Added `FluxABalanceFetcher` port and service dependency resolution.
- Added encrypted-credential-backed `Service.GetFluxABalance`.
- Added FluxA balance upstream proxy for `/api/status` and `/api/user/self`.
- Added bounded JSON reads, non-redirecting HTTP behavior, envelope validation, finite numeric validation, supported display-type validation, and safe 401/403 error mapping.
- Added upstream success and failure regression tests.

Verification:
- `env -u GOROOT go test ./internal/auth -run 'TestFluxABalance|TestListFluxAModel' -count=1` passed.
- `env -u GOROOT go test ./...` passed.
- `git diff --check` and `git show --check HEAD` passed.

Notes:
- The default `go` environment had a stale `GOROOT` pointing to a missing Go tool directory; tests required `env -u GOROOT`.
- Unrelated user changes remain in the worktree untouched.

## Review fix (2026-09-07)

Status: complete

Commit: `76c6508` (`fix: validate FluxA balance payload fields`)

TDD evidence:
- Added regression tests for missing and `null` `quota`, missing CNY `usd_exchange_rate`, and `null` CUSTOM `custom_currency_exchange_rate` before changing the decoder.
- The red run failed as intended: all four malformed payloads returned a nil error because Go decoded absent/null float fields as zero values.
- Switched upstream quota and conditional exchange-rate fields to `*float64`, rejecting absent/null values only where the selected display type requires the rate.
- Added successful USD, CUSTOM, and TOKENS coverage, plus unsupported-display-type rejection coverage.

Verification output:
- `env -u GOROOT go test ./internal/auth -run TestFluxABalance -count=1` → `ok .../internal/auth`.
- `env -u GOROOT go test ./internal/auth -run 'TestFluxABalance|TestListFluxAModel' -count=1` → `ok .../internal/auth 0.675s`.
- `git diff --check` and `git show --check 76c6508` completed without output/errors.

Concerns:
- None in the implementation. The pre-existing stale `GOROOT` note still applies; use `env -u GOROOT` for Go verification in this workspace.

## Review fix coverage completion (2026-09-08)

Status: complete

Implemented:
- Expanded the conditional exchange-rate regression matrix to reject both missing and explicit `null` CNY `usd_exchange_rate` values, and both missing and explicit `null` CUSTOM `custom_currency_exchange_rate` values.
- The prior review-fix commit (`76c6508`) already contained the presence-aware pointer decoding and its original failing-test evidence, so these additional cases were green when introduced.

Verification:
- `env -u GOROOT go test ./internal/auth -run TestFluxABalance -count=1` passed.
- `env -u GOROOT go test ./internal/auth -run 'TestFluxABalance|TestListFluxAModel' -count=1` passed.
- `git diff --check` passed.

Concerns:
- None.
