# FluxA Model Groups Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Retain a FluxA session securely on the backend and show only FluxA users their account group and available models in Settings.

**Architecture:** The Go backend encrypts FluxA bearer tokens at rest and proxies the fixed-site FluxA self and models APIs. The session DTO identifies FluxA sessions; the Expo app conditionally adds a read-only screen. The supplied Simple Icons SVG is rendered locally through react-native-svg.

**Tech Stack:** Go, Chi, pgx/sqlc/PostgreSQL, AES-GCM, React Native/Expo Router, TanStack Query, TypeScript.

## Global Constraints

- Only paid/free configured HTTPS FluxA origins can receive upstream credentials.
- FluxA passwords and access tokens never appear in logs, session payloads, or client storage.
- QQA_FLUXA_CREDENTIALS_KEY is required, base64 encoded, and exactly 32 bytes after decoding.
- Preserve and do not stage the user’s existing Google-login worktree edits.
- Do not modify custom model configuration; model groups are read-only.

---

### Task 1: Store encrypted FluxA credentials after login

**Files:**

- Create: server/migrations/000023_fluxa_credentials.sql
- Modify: server/internal/domain/auth.go, server/internal/auth/ports.go, server/internal/auth/service.go, server/internal/auth/fluxa.go, server/internal/config/config.go, server/.env.example
- Modify: server/internal/adapters/store/sql/auth.sql, server/internal/adapters/store/postgres_auth.go, server/internal/adapters/store/memory.go, server/internal/adapters/store/memory_auth.go, server/internal/app/auth.go, generated server/internal/db/auth.sql.go
- Test: server/internal/auth/fluxa_test.go, server/internal/config/config_test.go

**Interfaces:**

- Consumes: FluxACredentialAuthenticator.Login(ctx, site, username, password) (string, error).
- Produces: FluxACredentialStore with UpsertFluxACredential and GetFluxACredential; TokenPair.FluxASite pointer.

- [ ] **Step 1: Write the failing tests**

~~~go
func TestLoginWithFluxACredentialsEncryptsAndStoresOnlyAccessToken(t *testing.T) {
    service, memory := newFluxAServiceWithCredentialKey(t)
    _, err := service.LoginWithFluxACredentials(context.Background(), FluxASitePaid, " user ", " secret ")
    if err != nil { t.Fatalf("login: %v", err) }
    stored := memory.FluxACredential(t, "fluxa-user-id", FluxASitePaid)
    if strings.Contains(stored.TokenCiphertext, "secret") || strings.Contains(stored.TokenCiphertext, "upstream-token") { t.Fatal("credential stored as plaintext") }
}

func TestDecodeFluxACredentialsKeyRejectsMissingOrWrongLength(t *testing.T) {
    _, err := config.DecodeFluxACredentialsKey("")
    if err == nil { t.Fatal("missing key unexpectedly accepted") }
}
~~~

- [ ] **Step 2: Verify red**

Run: cd server && go test ./internal/auth ./internal/config -run 'Test(LoginWithFluxACredentialsEncrypts|DecodeFluxACredentialsKey)' -count=1

Expected: FAIL because storage and cipher support are absent.

- [ ] **Step 3: Implement the smallest secure persistence boundary**

~~~sql
CREATE TABLE fluxa_credentials (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  site TEXT NOT NULL CHECK (site IN ('paid', 'free')),
  token_ciphertext TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, site)
);
~~~

~~~go
type FluxACredential struct {
    UserID, TokenCiphertext string
    Site FluxASite
    CreatedAt, UpdatedAt time.Time
}
type FluxACredentialCipher interface {
    Encrypt(plaintext string, additionalData []byte) (string, error)
    Decrypt(ciphertext string, additionalData []byte) (string, error)
}
~~~

Decode the required 32-byte key in config. Use AES-GCM with random nonce, RawStdEncoding of nonce plus ciphertext, and user ID/site as additional data. Authenticate and verify before encrypting/upserting the upstream token. Set FluxASite only for the FluxA credential-login TokenPair. Add all needed Postgres and memory methods and regenerate sqlc.

- [ ] **Step 4: Verify green**

Run: cd server && sqlc generate && go test ./internal/auth ./internal/config -run 'Test(LoginWithFluxACredentialsEncrypts|DecodeFluxACredentialsKey)' -count=1

Expected: PASS; stored value is ciphertext and no credential text is retained.

- [ ] **Step 5: Commit**

Run: git add server/migrations/000023_fluxa_credentials.sql server/internal/domain/auth.go server/internal/auth server/internal/config server/internal/adapters/store server/internal/app/auth.go server/internal/db/auth.sql.go server/.env.example && git commit -m 'feat: retain encrypted FluxA credentials'

### Task 2: Retrieve and normalize FluxA groups and models

**Files:**

- Create: server/internal/auth/fluxa_models.go, server/internal/adapters/httpapi/fluxa_models_handler.go, server/internal/adapters/httpapi/fluxa_models_routes.go
- Modify: server/internal/auth/ports.go, server/internal/auth/service.go, server/internal/auth/fluxa.go, server/internal/adapters/httpapi/router.go
- Test: server/internal/auth/fluxa_models_test.go, server/internal/adapters/httpapi/fluxa_models_handler_test.go

**Interfaces:**

- Consumes: the credential store/cipher from Task 1 and currentUser request context.
- Produces: Service.ListFluxAModelGroups(ctx, userID, site) and GET /v1/fluxa/model-groups.

- [ ] **Step 1: Write failing service and route tests**

~~~go
func TestListFluxAModelGroupsUsesConfiguredOriginAndNormalizesPayload(t *testing.T) {
    groups, err := service.ListFluxAModelGroups(ctx, user.ID, FluxASiteFree)
    if err != nil { t.Fatalf("list groups: %v", err) }
    want := []FluxAModelGroup{{Name: "default", Models: []FluxAModel{{ID: "gpt-4o", Name: "GPT-4o"}}}}
    if !reflect.DeepEqual(groups, want) { t.Fatalf("groups = %#v, want %#v", groups, want) }
    if recordedAuthorization != "Bearer upstream-token" { t.Fatalf("authorization = %q", recordedAuthorization) }
}

func TestFluxAModelGroupsRejectsNonFluxAUser(t *testing.T) {
    response := performAuthorizedGet(router, "/v1/fluxa/model-groups", passwordUserToken)
    if response.Code != http.StatusForbidden { t.Fatalf("status = %d", response.Code) }
}
~~~

- [ ] **Step 2: Verify red**

Run: cd server && go test ./internal/auth ./internal/adapters/httpapi -run 'Test(ListFluxAModelGroups|FluxAModelGroups)' -count=1

Expected: FAIL because neither service nor endpoint exists.

- [ ] **Step 3: Implement the fixed-origin proxy**

~~~go
type FluxAModel struct { ID string; Name string }
type FluxAModelGroup struct { Name string; Models []FluxAModel }
func (s *Service) ListFluxAModelGroups(
    ctx context.Context, userID string, site FluxASite,
) ([]FluxAModelGroup, error)
~~~

Use the existing redirect-disabled ten-second client and fluxAOrigin allowlist. Decrypt the user/site credential and make bounded JSON calls to /api/user/self and /api/models. Trim, de-duplicate, and sort group/model names. Do not accept a site request parameter. Map no saved FluxA credential to 403, upstream 401/403 to a safe re-login-required 401, and malformed/network/non-success responses to 503. Do not include upstream bodies in API errors.

- [ ] **Step 4: Verify green and regression suite**

Run: cd server && go test ./internal/auth ./internal/adapters/httpapi -run 'Test(ListFluxAModelGroups|FluxAModelGroups)' -count=1 && go test ./...

Expected: PASS; only configured origins receive a bearer header.

- [ ] **Step 5: Commit**

Run: git add server/internal/auth/fluxa_models.go server/internal/auth/fluxa_models_test.go server/internal/auth/ports.go server/internal/auth/service.go server/internal/auth/fluxa.go server/internal/adapters/httpapi/fluxa_models_handler.go server/internal/adapters/httpapi/fluxa_models_routes.go server/internal/adapters/httpapi/fluxa_models_handler_test.go server/internal/adapters/httpapi/router.go && git commit -m 'feat: expose FluxA model groups'

### Task 3: Expose FluxA session state and typed app API

**Files:**

- Modify: server/internal/auth/session_result.go, server/internal/adapters/httpapi/api_auth_dto.go, server/internal/adapters/httpapi/fluxa_auth_test.go
- Modify: app/types/api.ts, app/lib/api.ts, app/store/auth.ts, app/lib/session.ts, app/tests/fluxa-flow.test.ts

**Interfaces:**

- Consumes: TokenPair.FluxASite and the Task 2 endpoint.
- Produces: User.fluxaSite optional property and api.listFluxAModelGroups(token).

- [ ] **Step 1: Write failing DTO and app request tests**

~~~ts
test('FluxA session persists its site but ordinary sessions do not', () => {
  assert.equal(fluxaSession.user.fluxaSite, 'paid');
  assert.equal(passwordSession.user.fluxaSite, undefined);
});

test('model group API sends only the Knowvia bearer token', async () => {
  await api.listFluxAModelGroups('knowvia-token');
  assert.equal(fetchCall.url.endsWith('/fluxa/model-groups'), true);
  assert.equal(fetchCall.headers.Authorization, 'Bearer knowvia-token');
});
~~~

- [ ] **Step 2: Verify red**

Run: cd app && npm test -- --test-name-pattern='FluxA session|model group API'

Expected: FAIL because FluxASite and the client wrapper are absent.

- [ ] **Step 3: Add backwards-compatible DTOs**

~~~ts
export interface FluxAModelGroup {
  name: string;
  models: Array<{id: string; name: string}>;
}
export interface User {
  id: string;
  username: string;
  displayName: string;
  fluxaSite?: FluxASite;
}
~~~

Map FluxASite only into FluxA credential-login sessions. The existing serialized SecureStore User field persists it automatically and remains compatible with old JSON. Add api.listFluxAModelGroups through the ordinary Knowvia request helper; it must not import a FluxA origin or token.

- [ ] **Step 4: Verify green**

Run: cd app && npm test && cd ../server && go test ./internal/adapters/httpapi -run TestFluxALogin -count=1

Expected: PASS.

- [ ] **Step 5: Commit**

Run: git add server/internal/auth/session_result.go server/internal/adapters/httpapi/api_auth_dto.go server/internal/adapters/httpapi/fluxa_auth_test.go app/types/api.ts app/lib/api.ts app/store/auth.ts app/lib/session.ts app/tests/fluxa-flow.test.ts && git commit -m 'feat: identify FluxA sessions in the app'

### Task 4: Add icon and FluxA-only Settings screen

**Files:**

- Create: app/components/FluxAIcon.tsx, app/app/(account)/fluxa-model-groups.tsx, app/modules/settings/screens/FluxAModelGroupsScreen.tsx
- Modify: app/modules/auth/screens/LoginScreen-en.tsx, app/modules/profile/screens/ProfileScreen.tsx, app/app/_layout.tsx, app/modules/settings/screens/index.ts, app/i18n/messages.ts
- Test: app/tests/fluxa-flow.test.ts

**Interfaces:**

- Consumes: User.fluxaSite and api.listFluxAModelGroups from Task 3.
- Produces: a FluxA-only Profile row and a read-only /fluxa-model-groups screen.

- [ ] **Step 1: Write failing UI tests**

~~~ts
test('FluxA login renders the local RetroArch SVG', () => {
  const login = readFileSync('modules/auth/screens/LoginScreen-en.tsx', 'utf8');
  assert.match(login, /<FluxAIcon/);
  assert.doesNotMatch(login, /cdn\.simpleicons\.org/);
});

test('model group settings are gated by fluxaSite', () => {
  const profile = readFileSync('modules/profile/screens/ProfileScreen.tsx', 'utf8');
  assert.match(profile, /user\?\.fluxaSite/);
  assert.match(profile, /router\.push\('\/fluxa-model-groups'\)/);
});
~~~

- [ ] **Step 2: Verify red**

Run: cd app && npm test -- --test-name-pattern='RetroArch|gated by fluxaSite'

Expected: FAIL because icon, route, and conditional row are absent.

- [ ] **Step 3: Implement local icon and screen**

~~~tsx
export function FluxAIcon({color}: {color: string}) {
  return <Svg viewBox="0 0 24 24"><Path fill={color} d="M6.84 5.76L8.4 7.68H5.28l-.72 2.88H2.64l.72-2.88H1.44L0 13.44h3.84l-.48 1.92h3.36L4.2 18.24h2.82l2.34-2.88h5.28l2.34 2.88h2.82l-2.52-2.88h3.36l-.48-1.92H24l-1.44-5.76h-1.92l.72 2.88h-1.92l-.72-2.88H15.6l1.56-1.92h-2.04l-1.68 1.92h-2.88L8.88 5.76zm.24 3.84H9v1.92H7.08zm7.925 0h1.92v1.92h-1.92Z" /></Svg>;
}
~~~

Use the SVG path supplied by the user, not the CDN URL. Replace only FluxA login’s mail icon. Add English, Simplified Chinese, Traditional Chinese, Japanese, and Korean strings for the feature. Register the route. The screen uses useQuery with a FluxA model-groups key, loading/error/retry/empty states, and sections that list models under each group. Add the Profile row only for paid/free FluxASite values.

- [ ] **Step 4: Verify green**

Run: cd app && npm test && npx tsc --noEmit

Expected: PASS. Manually verify FluxA users see the icon, row, and grouped models; other users see no row.

- [ ] **Step 5: Commit**

Run: git add app/components/FluxAIcon.tsx 'app/app/(account)/fluxa-model-groups.tsx' app/modules/settings/screens/FluxAModelGroupsScreen.tsx app/modules/auth/screens/LoginScreen-en.tsx app/modules/profile/screens/ProfileScreen.tsx app/app/_layout.tsx app/modules/settings/screens/index.ts app/i18n/messages.ts app/tests/fluxa-flow.test.ts && git commit -m 'feat: show FluxA model groups in settings'

### Task 5: Final verification

**Files:**

- Modify only files identified by failing checks in Tasks 1-4.

- [ ] **Step 1: Run all automated checks**

Run: cd server && go test ./... && cd ../app && npm test && npx tsc --noEmit

Expected: PASS with no credential in output or serialized sessions.

- [ ] **Step 2: Confirm diff boundaries**

Run: git status --short && git diff --check && git log --oneline -4

Expected: existing Google-login edits stay unstaged and feature commits include only FluxA work.

- [ ] **Step 3: Confirm deployment guidance**

Run: rg -n 'QQA_FLUXA_CREDENTIALS_KEY' server/.env.example docs/superpowers/specs/2026-09-06-fluxa-model-groups-design.md

Expected: the environment requirement appears in both files.
