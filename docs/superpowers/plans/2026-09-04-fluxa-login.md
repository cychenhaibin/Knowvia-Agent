# FluxA Login Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the app email-login form with site-gated native FluxA/New API login and exchange verified FluxA identities for normal Knowvia sessions.

**Architecture:** The app presents a native site selector and credential screen, calls the selected New API `/api/user/login`, then sends the returned access token plus a fixed site ID to Knowvia. The Go server validates that token against the matching New API `/api/user/self`, resolves a site-scoped auth identity, and issues the existing Knowvia session payload.

**Tech Stack:** Expo Router, React Native, React Query, Zustand, TypeScript, Go, Chi, existing memory/Postgres auth stores, `httptest`.

## Global Constraints

- Support only `https://fluxa.camila.qzz.io` (`paid`) and `https://free.camila.qzz.io` (`free`).
- Accounts are independent by site plus New API numeric user ID; never merge by username or email.
- Keep New API access tokens in memory only; never persist or log them.
- Reject unknown site IDs before any outbound request.
- Preserve existing Google and Microsoft login behavior.
- Do not add registration, password recovery, passkey, third-party OAuth, or native 2FA in this change.
- Use `apply_patch` for source edits and run focused tests before broader verification.

## File Map

- Modify `app/modules/auth/screens/LoginScreen-en.tsx`: replace the email form with the FluxA entry and route to the native FluxA login screen.
- Create `app/modules/auth/screens/FluxALoginScreen.tsx`: site selector, disabled-until-selected credential form, New API login, and Knowvia session exchange.
- Create `app/app/(auth)/fluxa-login.tsx`: Expo Router screen wrapper that reads the selected site route parameter.
- Modify `app/lib/api.ts`: add the typed New API login request and Knowvia FluxA exchange request; keep existing request behavior intact.
- Modify `app/i18n/messages.ts`: add FluxA labels and errors for every supported language dictionary.
- `app/app/_layout.tsx`: no change is expected; the existing `(auth)` stack group auto-discovers the new route, and this file must be changed only if Expo Router reports an unresolved screen during validation.
- Modify `server/internal/domain/auth.go`: add site-specific FluxA auth providers.
- Create `server/internal/auth/fluxa.go`: fixed-site mapping, New API identity DTO, HTTP verifier, and FluxA login orchestration.
- Modify `server/internal/auth/service.go`: inject the verifier and expose `LoginWithFluxA` through the auth service.
- Modify `server/internal/auth/ports.go`: define the FluxA verifier interface and service dependency.
- Modify `server/internal/adapters/httpapi/auth_handler.go`: decode and validate `/auth/fluxa` requests and map errors to safe HTTP responses.
- Modify `server/internal/adapters/httpapi/auth_routes.go`: register the `/auth/fluxa` route with existing auth routes.
- Modify `server/internal/config/config.go`: add optional server-owned paid/free origin overrides with the two fixed defaults.
- Create `server/internal/auth/fluxa_test.go`: verifier and identity-resolution unit tests.
- Create `server/internal/adapters/httpapi/fluxa_auth_test.go`: endpoint contract and validation tests.

### Task 1: Add typed New API and Knowvia client methods

**Files:**
- Modify `app/lib/api.ts` near the existing auth methods.
- Modify `app/types/api.ts`: add shared FluxA site and login-result types.

**Interfaces:**
- Produce `api.loginWithFluxA(site: FluxASite, username: string, password: string): Promise<FluxALoginResult>` for the selected New API origin.
- Produce `api.exchangeFluxASession(site: FluxASite, accessToken: string): Promise<SessionPayload>` for Knowvia `/auth/fluxa`.

- [ ] **Step 1: Define fixed site and response types.**

```ts
export type FluxASite = 'paid' | 'free';

export type FluxALoginResult = {
  accessToken?: string;
  require2FA?: boolean;
  message?: string;
  user?: { id: number; username: string; displayName?: string; email?: string };
};
```

- [ ] **Step 2: Write request helpers before implementation.** Add a site-to-origin map containing exactly the two HTTPS origins and a parser that accepts New API's `{success, message, data}` shape, requiring `data.access_token` for a complete login and preserving `require_2fa` as an incomplete result.

- [ ] **Step 3: Implement `loginWithFluxA`.** POST JSON to `${origin}/api/user/login`; do not use the Knowvia `request()` helper because this request targets an external origin. Convert non-2FA New API failures to an `Error` using the safe `message` field.

- [ ] **Step 4: Implement `exchangeFluxASession`.** POST `{site, accessToken}` to the existing Knowvia API base `/auth/fluxa`; do not persist `accessToken`.

- [ ] **Step 5: Run TypeScript validation.**

Run: `cd app && npx tsc --noEmit`
Expected: PASS.

- [ ] **Step 6: Commit.**

```bash
git add app/lib/api.ts app/types/api.ts
git commit -m "feat: add FluxA authentication clients"
```

### Task 2: Build native FluxA login screen and navigation

**Files:**
- Modify `app/modules/auth/screens/LoginScreen-en.tsx`.
- Create `app/modules/auth/screens/FluxALoginScreen.tsx`.
- Create `app/app/(auth)/fluxa-login.tsx`.
- Modify `app/i18n/messages.ts`.

**Interfaces:**
- Consume `FluxASite`, `api.loginWithFluxA`, and `api.exchangeFluxASession` from Task 1.
- Produce route `/ (auth) / fluxa-login` with an optional `site` parameter and a stored Knowvia session on success.

- [ ] **Step 1: Add pure site-state helpers before wiring JSX.** In `FluxALoginScreen.tsx`, keep site state typed as `FluxASite | null`; implement `canSubmit(site, username, password, pending)` and `nextPasswordAfterSiteChange(previousSite, nextSite)` as pure functions. The helper behavior is: no site or blank credentials or pending request returns `false`; changing between non-null sites returns an empty password; choosing the first site preserves an empty password. Record these cases in the manual checklist because this repo has no React Native test runner.

- [ ] **Step 2: Replace the email option.** In `LoginScreen-en.tsx`, remove the username/password email form and render one `FluxA Login` option that calls `router.push({ pathname: '/(auth)/fluxa-login', params: { openSitePicker: '1' } })`.

- [ ] **Step 3: Implement `FluxALoginScreen`.** Initialize `site` from the route parameter only when it is `paid` or `free`; otherwise leave it null and open the bottom sheet on mount. Render the site selector, username and secure password fields, submit button, loading state, and safe errors. Set `editable={site !== null}` and `disabled={!site || mutation.isPending}`. When the site changes, clear the password.

- [ ] **Step 4: Implement login sequence.** Call New API login, stop on `require2FA`, then call `api.exchangeFluxASession`; pass the returned Knowvia payload to `useAuthStore.getState().setSession`/hook setter and `router.replace('/(tabs)/runs')`. Keep the New API token in a local variable only.

- [ ] **Step 5: Add translations.** Add keys for FluxA login, title, site selector, paid/free labels, choose-site hint, site unavailable, verification failure, and 2FA-required message in English, Simplified Chinese, Traditional Chinese, Japanese, and Korean dictionaries.

- [ ] **Step 6: Run validation.**

Run: `cd app && npx tsc --noEmit`
Expected: PASS.

- [ ] **Step 7: Commit.**

```bash
git add app/modules/auth/screens/LoginScreen-en.tsx app/modules/auth/screens/FluxALoginScreen.tsx 'app/app/(auth)/fluxa-login.tsx' app/i18n/messages.ts
git commit -m "feat: add native FluxA login screen"
```

### Task 3: Implement server-side New API verifier

**Files:**
- Modify `server/internal/domain/auth.go`.
- Modify `server/internal/auth/ports.go`.
- Modify `server/internal/auth/service.go`.
- Create `server/internal/auth/fluxa.go`.
- Modify `server/internal/config/config.go`.

**Interfaces:**
- Produce `type FluxASite string` with values `FluxASitePaid` and `FluxASiteFree`.
- Produce `type FluxAIdentityVerifier interface { Verify(context.Context, FluxASite, string) (VerifiedFluxAIdentity, error) }`.
- Produce `func (s *Service) LoginWithFluxA(ctx context.Context, site FluxASite, accessToken string) (TokenPair, error)`.

- [ ] **Step 1: Write failing verifier tests.** Use an `httptest.Server` and a verifier configured with explicit origins to assert `/api/user/self`, bearer header, timeout-safe error, malformed payload rejection, and no call for an unknown site.

- [ ] **Step 2: Add error values and identity DTO.** Define `ErrFluxAInvalidToken`, `ErrFluxAUnavailable`, `ErrFluxAUnsupportedSite`, and `VerifiedFluxAIdentity{Subject, Username, DisplayName, Email, AvatarURL}`. Trim and require a positive numeric subject and non-empty username.

- [ ] **Step 3: Implement fixed site mapping.** Add `Config.FluxAPaidOrigin` from `QQA_FLUXA_PAID_ORIGIN` with default `https://fluxa.camila.qzz.io` and `Config.FluxAFreeOrigin` from `QQA_FLUXA_FREE_ORIGIN` with default `https://free.camila.qzz.io`. Map only `paid` and `free` to those server-owned values and reject any other value before constructing a request.

- [ ] **Step 4: Implement `Verify`.** Use an `http.Client` with a 10-second timeout, GET `/api/user/self`, set only `Authorization: Bearer <token>` and `Accept: application/json`, decode `{success,data:{id,username,...}}`, and return safe typed errors without response-body text.

- [ ] **Step 5: Inject the verifier.** Extend `Service` and `ServiceDeps`, keep `NewService` constructing the production verifier from config, and update the existing `NewServiceWithGoogleVerifier` and `NewServiceWithMicrosoftVerifier` constructors to pass the same production FluxA verifier so current tests continue to compile.

- [ ] **Step 6: Implement `LoginWithFluxA`.** Resolve the provider as `fluxa_paid` or `fluxa_free`, use provider subject equal to the New API numeric user ID, reuse the existing external-user allocation path, upsert profile fields and auth identity, ensure chat defaults, and issue the normal session. Never look up by username/email for identity matching.

- [ ] **Step 7: Run focused Go tests.**

Run: `cd server && go test ./internal/auth -run FluxA -v`
Expected: PASS.

- [ ] **Step 8: Commit.**

```bash
git add server/internal/domain/auth.go server/internal/auth/ports.go server/internal/auth/service.go server/internal/auth/fluxa.go server/internal/config/config.go
git commit -m "feat: verify FluxA identities server-side"
```

### Task 4: Expose `/v1/auth/fluxa` and test identity isolation

**Files:**
- Modify `server/internal/adapters/httpapi/auth_handler.go`.
- Modify `server/internal/adapters/httpapi/auth_routes.go`.
- Create `server/internal/adapters/httpapi/fluxa_auth_test.go`.
- Extend `server/internal/auth/fluxa_test.go` for paid/free isolation.

**Interfaces:**
- Consume `Service.LoginWithFluxA` from Task 3.
- Produce POST `/v1/auth/fluxa` accepting `{"site":"paid|free","accessToken":"..."}` and returning the existing session JSON shape.

- [ ] **Step 1: Write failing handler tests.** Assert valid paid request returns 200/session, missing fields return 400, unknown site returns 400 without verifier call, invalid upstream token returns 401, and paid/free same upstream ID produce different local user IDs.

- [ ] **Step 2: Implement request decoding and validation.** Trim fields, reject empty values, parse only `paid`/`free`, and never echo the access token.

- [ ] **Step 3: Map service errors.** Map unsupported site to 400, invalid token to 401, upstream unavailable to 503, and all other failures to 500 with a generic message. Reuse `writeSessionPayload` on success.

- [ ] **Step 4: Register the route.** Add `POST /auth/fluxa` alongside the existing login, Google, and Microsoft routes in `registerAuthRoutes`.

- [ ] **Step 5: Run HTTP tests and all server tests.**

Run: `cd server && go test ./internal/adapters/httpapi -run FluxA -v && go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit.**

```bash
git add server/internal/adapters/httpapi/auth_handler.go server/internal/adapters/httpapi/auth_routes.go server/internal/adapters/httpapi/fluxa_auth_test.go server/internal/auth/fluxa_test.go
git commit -m "feat: add FluxA auth exchange endpoint"
```

### Task 5: End-to-end verification and documentation

**Files:**
- Modify `docs/env.md`: document `QQA_FLUXA_PAID_ORIGIN` and `QQA_FLUXA_FREE_ORIGIN` and their fixed production defaults.
- Modify `server/.env.example`: add the two optional server-owned origin overrides.

- [ ] **Step 1: Run all app checks.**

Run: `cd app && npx tsc --noEmit`
Expected: PASS.

- [ ] **Step 2: Run all server checks.**

Run: `cd server && go test ./...`
Expected: PASS.

- [ ] **Step 3: Run a production-build check.**

Run: `cd app && npx expo export --platform web`
Expected: Expo exports the web bundle without module-resolution or route errors.

- [ ] **Step 4: Manually verify the login checklist.** Confirm the email entry is gone; FluxA opens the dedicated page; fields are visibly disabled before site selection; both site labels map to the intended hosts; changing sites clears password; successful exchange routes to runs; no token is logged or persisted.

- [ ] **Step 5: Update environment documentation and commit.** Document that clients still send only `paid` or `free`; the environment values are deployment-owned overrides and do not allow arbitrary client URLs.

```bash
git add docs/env.md server/.env.example
git commit -m "docs: document FluxA login origins"
```

## Self-Review Checklist

- [x] Covers native site selection and disabled credential state.
- [x] Covers both fixed New API origins and site-scoped identity isolation.
- [x] Covers client login, server verification, session issuance, route wiring, localization, and tests.
- [x] Contains no TBD/TODO/“implement later” placeholders.
- [x] Uses consistent names: `FluxASite`, `VerifiedFluxAIdentity`, `FluxAIdentityVerifier`, `LoginWithFluxA`.
- [x] Preserves existing auth providers and session payload format.
