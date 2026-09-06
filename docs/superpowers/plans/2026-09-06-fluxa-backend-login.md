# FluxA 后端单一登录流程 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 FluxA 账号密码认证完全迁移到 Knowvia 后端，App 只接收 Knowvia session。

**Architecture:** 后端新增站点限定的 FluxA 凭据认证器，在相同经过校验的 origin 上请求 `/api/user/login`，再复用现有身份验证和会话签发服务。`/v1/auth/fluxa` 只接受 `site`、`username`、`password`；客户端以单一 API 调用替换上游 token 的两段式流程。

**Tech Stack:** Go 标准库 HTTP、Go `httptest`、Expo/React Native、TypeScript、Node 内置测试运行器。

## Global Constraints

- App 不得再请求 FluxA 的两个域名或发送/接收上游 access token。
- 上游 token 和密码不得出现在响应或错误字符串中，也不得落库。
- `paid` 与 `free` 使用独立 provider 身份；仅允许固定的 HTTPS 纯 origin，拒绝重定向。
- `/v1/auth/fluxa` 只接受 `site`、`username`、`password`，成功响应维持现有 session DTO。

---

### Task 1: FluxA 后端凭据认证器

**Files:**
- Modify: `server/internal/auth/ports.go`
- Modify: `server/internal/auth/fluxa.go`
- Modify: `server/internal/auth/service.go`
- Modify: `server/internal/auth/fluxa_test.go`

**Interfaces:**
- Produces: `FluxACredentialAuthenticator.Login(context.Context, FluxASite, string, string) (string, error)`。
- Produces: `Service.LoginWithFluxACredentials(context.Context, FluxASite, string, string) (TokenPair, error)`。
- Consumes: existing `FluxAIdentityVerifier.Verify` and `LoginWithFluxA` to preserve identity and session rules.

- [ ] **Step 1: 写入失败 Go 测试**

为认证器添加 `httptest` 覆盖：付费/公益站分别收到 `POST /api/user/login` 和 JSON 账密；成功响应的 token 被传入 identity verifier；2FA、401、重定向、空 token 和网络故障映射为安全错误，且错误文本不含密码或 token。为服务添加 mock authenticator 断言凭据经 trim 后只调用一次并签发 session。

- [ ] **Step 2: 验证失败**

Run: `go test ./internal/auth -run 'FluxA.*Credential|LoginWithFluxACredentials'`

Expected: FAIL；认证器与凭据服务方法尚不存在。

- [ ] **Step 3: 实施最小服务端认证**

新增 `FluxACredentialAuthenticator`、安全错误 `ErrFluxAInvalidCredentials` 和 `ErrFluxA2FARequired`。复用 origin 校验、10 秒 timeout 和禁止重定向的 HTTP client；严格解析 New API 成功 JSON 中的 `data.access_token` / `data.require_2fa`。通过 `LoginWithFluxA` 验证 identity 并签发现有 session，立即丢弃 token。

- [ ] **Step 4: 验证 Go 测试通过**

Run: `go test ./internal/auth -run 'FluxA.*Credential|LoginWithFluxACredentials'`

Expected: PASS。

### Task 2: 单一后端 HTTP 合约

**Files:**
- Modify: `server/internal/adapters/httpapi/auth_handler.go:120-167`
- Modify: `server/internal/adapters/httpapi/fluxa_auth_test.go`

**Interfaces:**
- Consumes: `POST /v1/auth/fluxa` body `{site, username, password}`.
- Produces: existing `sessionDTO` on success; safe validation, unauthorized, 2FA, and unavailable responses otherwise.

- [ ] **Step 1: 写入失败 HTTP 测试**

替换 exchange 测试为 credential body；mock authenticator 断言收到已修剪用户名与原始密码，且其内部 token 不在响应。测试缺失 `username/password` 为 400、旧 `accessToken` body 为 400、非法站点不调用认证器、无效凭据为 401、2FA 使用明确安全响应。

- [ ] **Step 2: 验证失败**

Run: `go test ./internal/adapters/httpapi -run FluxA`

Expected: FAIL；handler 当前仍需要 `accessToken`。

- [ ] **Step 3: 实施合约迁移**

让 handler 读取 `site`、`username`、`password` 并调用 `LoginWithFluxACredentials`；删除 `accessToken` 验证。将错误安全映射为：无效凭据 401、2FA 401 且正文为 `complete two-factor authentication on the selected FluxA site`、上游不可用 503，且不回显敏感字段。

- [ ] **Step 4: 验证 HTTP 测试通过**

Run: `go test ./internal/adapters/httpapi -run FluxA`

Expected: PASS。

### Task 3: 客户端单一调用

**Files:**
- Modify: `app/lib/api.ts`
- Modify: `app/modules/auth/fluxaFlow.ts`
- Modify: `app/modules/auth/screens/FluxALoginScreen.tsx`
- Modify: `app/tests/fluxa-flow.test.ts`

**Interfaces:**
- Produces: `api.loginWithFluxA(site, username, password): Promise<SessionPayload>`，仅调用 Knowvia `/auth/fluxa`。
- Produces: `loginThroughFluxA(input, {loginWithFluxA, persistSession})`，不再暴露上游 token/exchange 依赖。

- [ ] **Step 1: 写入失败 TypeScript 测试**

将现有 two-stage flow test 改为断言 `loginWithFluxA` 接收 `free`、trim 后用户名和密码，直接返回 Knowvia session 并持久化；断言调用记录中无 origin、`accessToken` 或 `exchange`。保留未选站点和切站清空密码测试。

- [ ] **Step 2: 验证失败**

Run: `npm test -- --test-name-pattern="single backend FluxA login"`

Expected: FAIL；当前 flow 仍需要 `exchangeFluxASession` 和上游 token。

- [ ] **Step 3: 实施最小客户端迁移**

删除 origin 常量、origin 解析、上游响应解析和 exchange API；`api.loginWithFluxA` 用内部 `request` 提交 `{site, username, password}`。简化 flow dependencies 为单一 session 调用，页面只注入该调用与 session 持久化。

- [ ] **Step 4: 验证客户端测试通过**

Run: `npm test -- --test-name-pattern="single backend FluxA login" && npm test`

Expected: PASS。

### Task 4: 全量验证与提交

**Files:**
- Modify: all files from Tasks 1-3

- [ ] **Step 1: 运行服务端完整测试**

Run: `go test ./...`

Expected: exit code 0。

- [ ] **Step 2: 运行客户端完整测试和 Android Debug 构建**

Run: `npm test && (cd android && ./gradlew :app:assembleDebug)`

Expected: both exit code 0。

- [ ] **Step 3: 提交**

Run: `git add server/internal/auth server/internal/adapters/httpapi app/lib/api.ts app/modules/auth app/tests/fluxa-flow.test.ts && git commit -m 'feat: route FluxA login through backend'`
